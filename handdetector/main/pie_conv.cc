/*
 * CONV_2D and DEPTHWISE_CONV_2D on the ESP32-S3's PIE SIMD unit.
 *
 * Three paths, all int8 per-channel and bit-exact with TFLM's reference
 * kernels (same integer accumulator, then the same
 * MultiplyByQuantizedMultiplier, output offset and clamp):
 *
 *  pointwise  CONV_2D 1x1 stride 1, IC % 16 == 0 (29 layers, 74% of MACs).
 *             Every output is a dot product of one pixel's IC channels
 *             with one filter row: EE.VMULAS.S8.ACCX sums 16 int8 products
 *             into the 40-bit ACCX accumulator.
 *  im2col     any other CONV_2D (here: the first 5x5 stride-2, 3->32).
 *             Each output pixel's receptive field (KH*KW*IC bytes, padded
 *             to a multiple of 16) is gathered into a row, then the same
 *             ACCX dot product against a filter row.
 *  depthwise  DEPTHWISE_CONV_2D, multiplier 1, C % 16 == 0 (23 layers).
 *             16 channels at a time: EE.VMULAS.S8.QACC multiplies lane by
 *             lane into 16 separate 20-bit QACC accumulators, one per tap.
 *             The input is first copied into a zero-point-padded buffer so
 *             every tap is in bounds and 16-byte aligned.
 *
 * Input offset folding: reference acc = sum(w * (x + off)) + bias with
 * out-of-image taps skipped. Out-of-image taps here read the zero point,
 * so w * (zp + off) = 0 and they contribute nothing either way; then
 * sum(w * (x + off)) = sum(w * x) + off * sum(w), and off * sum(w) is
 * folded into a per-channel bias once per layer.
 *
 * Range: |w * x| <= 128 * 128 = 2^14. ACCX is 40 bits (plenty); a QACC
 * lane is 20 bits signed (saturating), enough for 2^19 / 2^14 = 32 taps,
 * so depthwise is limited to KH*KW <= 32 (5x5 = 25).
 *
 * Alignment: PIE loads ignore the low 4 address bits. Activations come
 * from the TFLM arena (16-byte aligned) or our own aligned buffers;
 * weights live in memory-mapped flash with no alignment guarantee, so
 * they are copied once per layer into aligned PSRAM.
 */
#include <stdio.h>
#include <string.h>

#include "freertos/FreeRTOS.h"
#include "esp_cpu.h"
#include "esp_heap_caps.h"
#include "esp_timer.h"

#include "tensorflow/lite/micro/kernels/conv.h"
#include "tensorflow/lite/micro/kernels/depthwise_conv.h"
#include "tensorflow/lite/micro/kernels/kernel_util.h"
#include "tensorflow/lite/micro/kernels/internal/common.h"

#include "pie_kernels.h"
#include "pie_parallel.h"

namespace {

using namespace tflite;

TFLMRegistration ref_conv;  /* TFLM's reference kernels */
TFLMRegistration ref_dw;
bool enabled = true;
bool verify = false;

enum Kind : uint8_t { REF, POINTWISE, IM2COL, DEPTHWISE };
const char *const kind_name[] = { "ref", "pw", "im2col", "dw" };

/* Per-layer state, keyed by node (nodes are stable for the interpreter's
 * lifetime). */
struct Layer {
	const TfLiteNode *node;
	bool dw_op;       /* DEPTHWISE_CONV_2D node (else CONV_2D) */
	bool prepared;
	int ic, oc;       /* depthwise: ic == oc == channels */
	int kh, kw;
	int k16;          /* conv: row length (KH*KW*IC rounded up to 16) */
	int8_t *w;        /* conv: [oc][k16]; depthwise: [kh*kw][c]; aligned */
	int32_t *bias;    /* bias + input_offset * sum(w), per output channel */
	/* last inference */
	Kind kind;
	int64_t us;
	int mismatches;   /* verify mode */
};
constexpr int kMaxLayers = 64;
Layer layers[kMaxLayers];
int n_layers;

Layer *find_layer(const TfLiteNode *node, bool dw_op)
{
	for (int i = 0; i < n_layers; i++)
		if (layers[i].node == node) return &layers[i];
	if (n_layers == kMaxLayers) return nullptr;
	layers[n_layers] = Layer{};
	layers[n_layers].node = node;
	layers[n_layers].dw_op = dw_op;
	return &layers[n_layers++];
}

/* One scratch buffer shared by all layers (inference is single-threaded):
 * the padded input for depthwise, one im2col row for conv. Internal RAM
 * if it fits, PSRAM otherwise. */
int8_t *scratch;
size_t scratch_size;

int8_t *get_scratch(size_t n)
{
	if (n <= scratch_size) return scratch;
	heap_caps_free(scratch);
	scratch = (int8_t *)heap_caps_aligned_alloc(16, n, MALLOC_CAP_INTERNAL | MALLOC_CAP_8BIT);
	if (!scratch) scratch = (int8_t *)heap_caps_aligned_alloc(16, n, MALLOC_CAP_SPIRAM);
	scratch_size = scratch ? n : 0;
	return scratch;
}

inline int32_t requant(int32_t acc, int32_t mult, int32_t shift, int32_t out_off, int32_t lo, int32_t hi)
{
	acc = pie_mbqm(acc, mult, shift) + out_off;
	return acc < lo ? lo : acc > hi ? hi : acc;
}

/* ---- ACCX dot product (pointwise, im2col) ---- */

/* 16*n int8 dot product. x and w 16-byte aligned. Sums stay far below
 * 2^31, so ACCX's low 32 bits (RUR.ACCX_0) are the exact result. */
inline int32_t dot_s8(const int8_t *x, const int8_t *w, int n16)
{
	int32_t r;
	asm volatile("ee.zero.accx");
	for (int k = 0; k < n16; k++) {
		asm volatile(
			"ee.vld.128.ip q0, %0, 16\n\t"
			"ee.vld.128.ip q1, %1, 16\n\t"
			"ee.vmulas.s8.accx q0, q1\n\t"
			: "+a"(x), "+a"(w)
			:
			: "memory");
	}
	asm volatile("rur.accx_0 %0" : "=a"(r));
	return r;
}

/* x . w[o] for every output channel o of one pixel. */
inline void dot_all(const Layer *L, const micro::OpDataConv &d, const int8_t *x, int n16, int8_t *out)
{
	const int8_t *w = L->w;
	const int stride = n16 * 16;
	for (int o = 0; o < L->oc; o++, w += stride)
		*out++ = (int8_t)requant(dot_s8(x, w, n16) + L->bias[o],
					 d.per_channel_output_multiplier[o], d.per_channel_output_shift[o],
					 d.output_zero_point, d.output_activation_min, d.output_activation_max);
}

/* ---- CONV_2D ---- */

Kind conv_kind(const TfLiteConvParams &p, const TfLiteEvalTensor *in,
	       const TfLiteEvalTensor *filter, const TfLiteEvalTensor *out)
{
	if (in->type != kTfLiteInt8 || filter->type != kTfLiteInt8 || out->type != kTfLiteInt8) return REF;
	if (p.dilation_width_factor != 1 || p.dilation_height_factor != 1) return REF;
	if (in->dims->data[0] != 1) return REF;
	const TfLiteIntArray *f = filter->dims;  /* OHWI */
	if (f->size != 4 || in->dims->data[3] != f->data[3]) return REF;
	int ic = f->data[3];
	if (f->data[1] == 1 && f->data[2] == 1 && p.stride_width == 1 && p.stride_height == 1 &&
	    ic % 16 == 0 && ((uintptr_t)in->data.int8 & 15) == 0)
		return POINTWISE;
	return IM2COL;
}

bool prepare_conv(Layer *L, Kind kind, const TfLiteEvalTensor *filter, const TfLiteEvalTensor *bias,
		  const micro::OpDataConv &d)
{
	L->oc = filter->dims->data[0];
	L->kh = filter->dims->data[1];
	L->kw = filter->dims->data[2];
	L->ic = filter->dims->data[3];
	const int k = L->kh * L->kw * L->ic;
	L->k16 = (k + 15) & ~15;
	L->w = (int8_t *)heap_caps_aligned_alloc(16, (size_t)L->oc * L->k16, MALLOC_CAP_SPIRAM);
	L->bias = (int32_t *)heap_caps_malloc(L->oc * sizeof(int32_t), MALLOC_CAP_SPIRAM);
	if (!L->w || !L->bias) return false;
	const int32_t input_offset = -d.input_zero_point;
	for (int o = 0; o < L->oc; o++) {
		int8_t *row = L->w + (size_t)o * L->k16;
		memcpy(row, filter->data.int8 + (size_t)o * k, k);  /* OHWI row = (ky, kx, c) order */
		memset(row + k, 0, L->k16 - k);
		int32_t s = 0;
		for (int i = 0; i < k; i++) s += row[i];
		L->bias[o] = (bias ? bias->data.i32[o] : 0) + input_offset * s;
	}
	(void)kind;
	return true;
}

/* Blocking for the 32 KB data cache: going pixel by pixel over all output
 * channels re-reads every weight row for every pixel, and a 256x256 layer
 * (64 KB of weights) then streams them from PSRAM per pixel (~1075 vs ~240
 * cycles per output). So the output channels are split into blocks whose
 * weights take at most kWeightBlock bytes; each block stays cached while
 * all pixels pass through it. Small layers are a single block, i.e. plain
 * pixel order. */
constexpr size_t kWeightBlock = 16 * 1024;

void pointwise_range(const Layer *L, const micro::OpDataConv &d, const int8_t *in, int8_t *out, int p0, int p1)
{
	const int ic = L->ic, oc = L->oc, n16 = ic / 16;
	int block = (int)(kWeightBlock / ic);
	if (block < 1) block = 1;
	if (block >= oc) {
		for (int p = p0; p < p1; p++)
			dot_all(L, d, in + (size_t)p * ic, n16, out + (size_t)p * oc);
		return;
	}
	const int32_t out_off = d.output_zero_point, lo = d.output_activation_min, hi = d.output_activation_max;
	const int32_t *mult = d.per_channel_output_multiplier, *shift = d.per_channel_output_shift;
	for (int o0 = 0; o0 < oc; o0 += block) {
		const int o1 = o0 + block < oc ? o0 + block : oc;
		const int8_t *x = in + (size_t)p0 * ic;
		int8_t *y = out + (size_t)p0 * oc;
		for (int p = p0; p < p1; p++, x += ic, y += oc) {
			const int8_t *w = L->w + (size_t)o0 * ic;
			for (int o = o0; o < o1; o++, w += ic)
				y[o] = (int8_t)requant(dot_s8(x, w, n16) + L->bias[o], mult[o], shift[o], out_off, lo, hi);
		}
	}
}

void conv_pointwise(const Layer *L, const micro::OpDataConv &d, const int8_t *in, int8_t *out, int pixels)
{
	pie_parallel_for(pixels, [&](int p0, int p1) { pointwise_range(L, d, in, out, p0, p1); });
}

bool conv_im2col(const Layer *L, const TfLiteConvParams &p, const micro::OpDataConv &d,
		 const TfLiteEvalTensor *in, TfLiteEvalTensor *out)
{
	const int ih = in->dims->data[1], iw = in->dims->data[2], ic = L->ic;
	const int oh = out->dims->data[1], ow = out->dims->data[2];
	const int sh = p.stride_height, sw = p.stride_width;
	const int ph = d.padding.height, pw = d.padding.width;
	const int8_t zp = (int8_t)d.input_zero_point;
	const int row_bytes = L->kw * ic;
	/* one gathered row per core; the tail stays 0 (its weights are 0 too) */
	int8_t *cols = get_scratch(2 * (size_t)L->k16);
	if (!cols) return false;
	memset(cols, 0, 2 * (size_t)L->k16);
	const int8_t *src = in->data.int8;
	pie_parallel_for(oh, [&](int oy0, int oy1) {
	int8_t *col = cols + (xPortGetCoreID() ? L->k16 : 0);
	int8_t *dst = out->data.int8 + (size_t)oy0 * ow * L->oc;
	for (int oy = oy0; oy < oy1; oy++) {
		for (int ox = 0; ox < ow; ox++, dst += L->oc) {
			const int y0 = oy * sh - ph, x0 = ox * sw - pw;
			int8_t *c = col;
			for (int ky = 0; ky < L->kh; ky++, c += row_bytes) {
				const int y = y0 + ky;
				if (y < 0 || y >= ih) { memset(c, zp, row_bytes); continue; }
				const int xa = x0 < 0 ? 0 : x0, xb = x0 + L->kw > iw ? iw : x0 + L->kw;
				if (xa > x0) memset(c, zp, (xa - x0) * ic);
				memcpy(c + (xa - x0) * ic, src + ((size_t)y * iw + xa) * ic, (xb - xa) * ic);
				if (x0 + L->kw > xb) memset(c + (xb - x0) * ic, zp, (x0 + L->kw - xb) * ic);
			}
			dot_all(L, d, col, L->k16 / 16, dst);
		}
	}
	});
	return true;
}

/* ---- DEPTHWISE_CONV_2D ---- */

bool dw_eligible(const TfLiteDepthwiseConvParams &p, const TfLiteEvalTensor *in,
		 const TfLiteEvalTensor *filter, const TfLiteEvalTensor *out)
{
	if (in->type != kTfLiteInt8 || filter->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (p.dilation_width_factor != 1 || p.dilation_height_factor != 1) return false;
	if (p.depth_multiplier != 1 || in->dims->data[0] != 1) return false;
	const TfLiteIntArray *f = filter->dims;  /* 1, KH, KW, C */
	if (f->size != 4 || f->data[0] != 1 || f->data[3] != in->dims->data[3]) return false;
	if (f->data[3] % 16 != 0 || f->data[1] * f->data[2] > 32) return false;
	return true;
}

bool prepare_dw(Layer *L, const TfLiteEvalTensor *filter, const TfLiteEvalTensor *bias,
		const micro::OpDataConv &d)
{
	L->kh = filter->dims->data[1];
	L->kw = filter->dims->data[2];
	L->ic = L->oc = filter->dims->data[3];
	const size_t n = (size_t)L->kh * L->kw * L->oc;
	L->w = (int8_t *)heap_caps_aligned_alloc(16, n, MALLOC_CAP_SPIRAM);
	L->bias = (int32_t *)heap_caps_malloc(L->oc * sizeof(int32_t), MALLOC_CAP_SPIRAM);
	if (!L->w || !L->bias) return false;
	memcpy(L->w, filter->data.int8, n);  /* [ky][kx][c] */
	const int32_t input_offset = -d.input_zero_point;
	for (int c = 0; c < L->oc; c++) {
		int32_t s = 0;
		for (int t = 0; t < L->kh * L->kw; t++) s += L->w[t * L->oc + c];
		L->bias[c] = (bias ? bias->data.i32[c] : 0) + input_offset * s;
	}
	return true;
}

/* Lane i of a 160-bit QACC half stored little-endian at b: bits
 * [20i+19 : 20i], signed. (TRM: EE.VMULAS.S8.QACC lane layout.) */
inline int32_t lane20(const uint8_t *b, int i)
{
	const int bit = 20 * i;
	uint32_t v;
	memcpy(&v, b + (bit >> 3), 4);
	return (int32_t)((v >> (bit & 7)) << 12) >> 12;
}

bool dw_conv(const Layer *L, const TfLiteDepthwiseConvParams &p, const micro::OpDataConv &d,
	     const TfLiteEvalTensor *in, TfLiteEvalTensor *out)
{
	const int ih = in->dims->data[1], iw = in->dims->data[2], C = L->oc;
	const int oh = out->dims->data[1], ow = out->dims->data[2];
	const int sh = p.stride_height, sw = p.stride_width;
	const int ph = d.padding.height, pw = d.padding.width;
	const int8_t zp = (int8_t)d.input_zero_point;

	/* Padded input: covers every tap of every output, zero point outside
	 * the image. Rows are PW*C bytes, a multiple of 16. */
	const int PH = (oh - 1) * sh + L->kh, PW = (ow - 1) * sw + L->kw;
	const size_t prow = (size_t)PW * C;
	int8_t *pad = get_scratch((size_t)PH * prow);
	if (!pad) return false;
	for (int y = 0; y < PH; y++) {
		int8_t *r = pad + y * prow;
		const int iy = y - ph;
		if (iy < 0 || iy >= ih) { memset(r, zp, prow); continue; }
		const int xa = pw, xb = pw + iw < PW ? pw + iw : PW;  /* padded x range holding image */
		memset(r, zp, (size_t)xa * C);
		memcpy(r + (size_t)xa * C, in->data.int8 + (size_t)iy * iw * C, (size_t)(xb - xa) * C);
		if (xb < PW) memset(r + (size_t)xb * C, zp, (size_t)(PW - xb) * C);
	}

	const int32_t *mult = d.per_channel_output_multiplier, *shift = d.per_channel_output_shift;
	const int32_t out_off = d.output_zero_point, lo = d.output_activation_min, hi = d.output_activation_max;
	const int step = C;  /* next tap along x, both in input and weights */
	pie_parallel_for(oh, [&](int oy0, int oy1) {
	alignas(16) uint8_t acc[64];
	int8_t *dst = out->data.int8 + (size_t)oy0 * ow * C;
	for (int oy = oy0; oy < oy1; oy++) {
		for (int ox = 0; ox < ow; ox++) {
			const int8_t *base = pad + (size_t)oy * sh * prow + (size_t)ox * sw * C;
			for (int g = 0; g < C; g += 16, dst += 16) {
				asm volatile("ee.zero.qacc");
				for (int ky = 0; ky < L->kh; ky++) {
					const int8_t *x = base + ky * prow + g;
					const int8_t *w = L->w + (size_t)ky * L->kw * C + g;
					for (int kx = 0; kx < L->kw; kx++) {
						asm volatile(
							"ee.vld.128.xp q0, %0, %2\n\t"
							"ee.vld.128.xp q1, %1, %2\n\t"
							"ee.vmulas.s8.qacc q0, q1\n\t"
							: "+a"(x), "+a"(w)
							: "a"(step)
							: "memory");
					}
				}
				/* QACC_L = lanes 0..7 -> acc[0..20), QACC_H = lanes 8..15 -> acc[32..52) */
				uint8_t *a = acc;
				asm volatile(
					"ee.st.qacc_l.l.128.ip %0, 16\n\t"
					"ee.st.qacc_l.h.32.ip %0, 16\n\t"
					"ee.st.qacc_h.l.128.ip %0, 16\n\t"
					"ee.st.qacc_h.h.32.ip %0, 0\n\t"
					: "+a"(a) : : "memory");
				for (int i = 0; i < 16; i++) {
					const int c = g + i;
					int32_t v = (i < 8 ? lane20(acc, i) : lane20(acc + 32, i - 8)) + L->bias[c];
					dst[i] = (int8_t)requant(v, mult[c], shift[c], out_off, lo, hi);
				}
			}
		}
	}
	});
	return true;
}

/* ---- dispatch ---- */

/* Run the reference kernel into the output and count bytes that differ
 * from what the PIE path wrote. */
int verify_against(const TFLMRegistration &ref, TfLiteContext *ctx, TfLiteNode *node, TfLiteEvalTensor *out)
{
	size_t n = 1;
	for (int i = 0; i < out->dims->size; i++) n *= out->dims->data[i];
	int8_t *mine = (int8_t *)heap_caps_malloc(n, MALLOC_CAP_SPIRAM);
	if (!mine) return -1;
	memcpy(mine, out->data.int8, n);
	ref.invoke(ctx, node);
	int bad = 0;
	for (size_t i = 0; i < n; i++) bad += mine[i] != out->data.int8[i];
	heap_caps_free(mine);
	return bad;
}

TfLiteStatus ConvEval(TfLiteContext *ctx, TfLiteNode *node)
{
	const auto &params = *static_cast<const TfLiteConvParams *>(node->builtin_data);
	const auto &data = *static_cast<const micro::OpDataConv *>(node->user_data);
	const TfLiteEvalTensor *in = micro::GetEvalInput(ctx, node, micro::kConvInputTensor);
	const TfLiteEvalTensor *filter = micro::GetEvalInput(ctx, node, micro::kConvWeightsTensor);
	const TfLiteEvalTensor *bias = node->inputs->size == 3
		? micro::GetEvalInput(ctx, node, micro::kConvBiasTensor) : nullptr;
	TfLiteEvalTensor *out = micro::GetEvalOutput(ctx, node, micro::kConvOutputTensor);

	Layer *L = find_layer(node, false);
	int64_t t0 = esp_timer_get_time();
	Kind kind = enabled && L ? conv_kind(params, in, filter, out) : REF;
	if (kind != REF && !L->prepared) {
		L->prepared = true;
		if (!prepare_conv(L, kind, filter, bias, data)) { heap_caps_free(L->w); L->w = nullptr; }
	}
	if (kind != REF && !L->w) kind = REF;
	bool ok = true;
	if (kind == POINTWISE)
		conv_pointwise(L, data, in->data.int8, out->data.int8, out->dims->data[1] * out->dims->data[2]);
	else if (kind == IM2COL)
		ok = conv_im2col(L, params, data, in, out);
	if (kind == REF || !ok) {
		TfLiteStatus s = ref_conv.invoke(ctx, node);
		if (L) { L->kind = REF; L->us = esp_timer_get_time() - t0; L->mismatches = 0; }
		return s;
	}
	L->kind = kind;
	L->us = esp_timer_get_time() - t0;
	L->mismatches = verify ? verify_against(ref_conv, ctx, node, out) : 0;
	return kTfLiteOk;
}

TfLiteStatus DwEval(TfLiteContext *ctx, TfLiteNode *node)
{
	const auto &params = *static_cast<const TfLiteDepthwiseConvParams *>(node->builtin_data);
	const auto &data = *static_cast<const micro::OpDataConv *>(node->user_data);
	const TfLiteEvalTensor *in = micro::GetEvalInput(ctx, node, kDepthwiseConvInputTensor);
	const TfLiteEvalTensor *filter = micro::GetEvalInput(ctx, node, kDepthwiseConvWeightsTensor);
	const TfLiteEvalTensor *bias = node->inputs->size == 3
		? micro::GetEvalInput(ctx, node, kDepthwiseConvBiasTensor) : nullptr;
	TfLiteEvalTensor *out = micro::GetEvalOutput(ctx, node, kDepthwiseConvOutputTensor);

	Layer *L = find_layer(node, true);
	int64_t t0 = esp_timer_get_time();
	bool pie = enabled && L && dw_eligible(params, in, filter, out);
	if (pie && !L->prepared) {
		L->prepared = true;
		if (!prepare_dw(L, filter, bias, data)) { heap_caps_free(L->w); L->w = nullptr; }
	}
	if (pie && (!L->w || !dw_conv(L, params, data, in, out))) pie = false;
	if (!pie) {
		TfLiteStatus s = ref_dw.invoke(ctx, node);
		if (L) { L->kind = REF; L->us = esp_timer_get_time() - t0; L->mismatches = 0; }
		return s;
	}
	L->kind = DEPTHWISE;
	L->us = esp_timer_get_time() - t0;
	L->mismatches = verify ? verify_against(ref_dw, ctx, node, out) : 0;
	return kTfLiteOk;
}

}  // namespace

TFLMRegistration PieRegister_CONV_2D()
{
	ref_conv = tflite::Register_CONV_2D();
	TFLMRegistration r = ref_conv;
	r.invoke = ConvEval;
	return r;
}

TFLMRegistration PieRegister_DEPTHWISE_CONV_2D()
{
	ref_dw = tflite::Register_DEPTHWISE_CONV_2D();
	TFLMRegistration r = ref_dw;
	r.invoke = DwEval;
	return r;
}

extern "C" void pie_set_enabled(bool on) { enabled = on; }
extern "C" bool pie_enabled(void) { return enabled; }
extern "C" void pie_set_verify(bool on) { verify = on; }
extern "C" bool pie_verify_enabled(void) { return verify; }

/* `det pie bench`: cycle counts of the building blocks, to see where
 * the per-output time goes. */
extern "C" void pie_microbench(void)
{
	const int N = 4096;
	int32_t *acc = (int32_t *)heap_caps_malloc(N * 4, MALLOC_CAP_INTERNAL);
	int8_t *wi = (int8_t *)heap_caps_aligned_alloc(16, 65536, MALLOC_CAP_INTERNAL | MALLOC_CAP_8BIT);
	int8_t *wp = (int8_t *)heap_caps_aligned_alloc(16, 65536, MALLOC_CAP_SPIRAM);
	int8_t *flush = (int8_t *)heap_caps_aligned_alloc(16, 131072, MALLOC_CAP_SPIRAM);
	alignas(16) static int8_t x[256];
	if (!acc || !wp || !flush) { printf("alloc failed\n"); goto out; }
	for (int i = 0; i < N; i++) acc[i] = (i * 7919) % 200000 - 100000;
	for (int i = 0; i < 65536; i++) { wp[i] = (int8_t)(i * 31); if (wi) wi[i] = wp[i]; }
	for (int i = 0; i < 256; i++) x[i] = (int8_t)(i * 13);
	{
		volatile int32_t sink = 0;
		uint32_t c0 = esp_cpu_get_cycle_count();
		for (int i = 0; i < N; i++) sink = sink + requant(acc[i], 1518500250, -7, -3, -128, 127);
		uint32_t c1 = esp_cpu_get_cycle_count();
		printf("requant, pie_mbqm + offset + clamp: %.1f cycles\n", (float)(c1 - c0) / N);
		c0 = esp_cpu_get_cycle_count();
		for (int i = 0; i < N; i++) sink = sink + tflite::micro::MultiplyByQuantizedMultiplier(acc[i], 1518500250, -7);
		c1 = esp_cpu_get_cycle_count();
		printf("TFLM MultiplyByQuantizedMultiplier alone: %.1f cycles\n", (float)(c1 - c0) / N);

		/* exactness: random accumulators, multipliers and shifts in the
		 * ranges TFLM produces (M in [2^30, 2^31), shift -31..7) */
		uint32_t r = 12345, bad = 0, n = 0;
		auto rnd = [&r]() { r = r * 1664525u + 1013904223u; return r; };
		for (int i = 0; i < 400000; i++) {
			const int shift = (int)(rnd() % 39) - 31;
			const int32_t m = (int32_t)((1u << 30) | (rnd() >> 2));
			const int bits = shift > 0 ? 23 - shift : 30;  /* keep x << shift in range */
			const int32_t x = (int32_t)(rnd() >> (32 - bits)) - (1 << (bits - 1));
			bad += pie_mbqm(x, m, shift) != tflite::micro::MultiplyByQuantizedMultiplier(x, m, shift);
			n++;
		}
		printf("pie_mbqm vs TFLM on %lu random inputs: %lu mismatches\n", (unsigned long)n, (unsigned long)bad);

		for (int n16 : { 2, 4, 8, 16 }) {
			c0 = esp_cpu_get_cycle_count();
			for (int i = 0; i < N; i++) sink = sink + dot_s8(x, wi ? wi + (i & 15) * 256 : wp, n16);
			c1 = esp_cpu_get_cycle_count();
			printf("dot_s8 %3d channels, cached weights: %.1f cycles\n", n16 * 16, (float)(c1 - c0) / N);
		}
		/* one 256->256 pointwise pixel: 256 dot products over 64 KB of weights */
		for (int pass = 0; pass < 2; pass++) {
			int8_t *w = pass ? wi : wp;
			if (!w) continue;
			for (int i = 0; i < 131072; i += 32) sink = sink + flush[i];  /* evict */
			c0 = esp_cpu_get_cycle_count();
			for (int o = 0; o < 256; o++) sink = sink + dot_s8(x, w + o * 256, 16);
			c1 = esp_cpu_get_cycle_count();
			printf("256->256 pixel, weights in %s (cold): %.1f cycles/output\n", pass ? "internal RAM" : "PSRAM", (float)(c1 - c0) / 256);
		}
		/* streaming read bandwidth: 16-byte PIE loads over 64 KB */
		for (int pass = 0; pass < 2; pass++) {
			const int8_t *p = pass ? wi : wp;
			if (!p) continue;
			for (int i = 0; i < 131072; i += 32) sink = sink + flush[i];
			c0 = esp_cpu_get_cycle_count();
			for (int i = 0; i < 4096; i++)
				asm volatile("ee.vld.128.ip q0, %0, 16" : "+a"(p) :: "memory");
			c1 = esp_cpu_get_cycle_count();
			printf("read 64 KB from %s (cold): %.2f cycles/16 B = %.0f MB/s\n", pass ? "internal RAM" : "PSRAM",
			       (float)(c1 - c0) / 4096, 65536.0 * 240 / (c1 - c0));
		}
		(void)sink;
	}
out:
	heap_caps_free(acc); heap_caps_free(wi); heap_caps_free(wp); heap_caps_free(flush);
}

extern "C" void pie_print_stats(void)
{
	int64_t us[4] = { 0 };
	int count[4] = { 0 }, bad = 0;
	printf("%-3s %-5s %-7s %10s %10s %s\n", "#", "op", "path", "shape", "ms", verify ? "mismatches" : "");
	for (int i = 0; i < n_layers; i++) {
		const Layer &L = layers[i];
		us[L.kind] += L.us;
		count[L.kind]++;
		bad += L.mismatches;
		char sh[24] = "-";
		if (L.w && L.dw_op) snprintf(sh, sizeof(sh), "%dx%d %d", L.kh, L.kw, L.oc);
		else if (L.w) snprintf(sh, sizeof(sh), "%dx%d %d->%d", L.kh, L.kw, L.ic, L.oc);
		printf("%-3d %-5s %-7s %10s %10.2f", i, L.dw_op ? "dw" : "conv", kind_name[L.kind], sh, L.us / 1e3);
		if (verify && L.kind != REF) printf(" %d", L.mismatches);
		printf("\n");
	}
	for (int k = 0; k < 4; k++)
		if (count[k]) printf("%-9s %2d layers %10.1f ms\n", kind_name[k], count[k], us[k] / 1e3);
	if (verify) printf("%s\n", bad ? "VERIFY MISMATCH" : "verify: all PIE layers bit-exact");
	pie_elementwise_print_stats();
	pie_parallel_print_stats();
}
