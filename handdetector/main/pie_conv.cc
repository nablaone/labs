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
#include "esp_memory_utils.h"
#include "esp_timer.h"

#include "tensorflow/lite/micro/kernels/conv.h"
#include "tensorflow/lite/micro/kernels/depthwise_conv.h"
#include "tensorflow/lite/micro/kernels/kernel_util.h"
#include "tensorflow/lite/micro/kernels/internal/common.h"

#include "pie_kernels.h"
#include "pie_parallel.h"
#include "pie_fusion.h"

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
	int32_t *bias;    /* conv: bias + input_offset * sum(w), per output channel */
	struct DwChan *dwq;  /* depthwise: per-channel bias and requant params */
	bool dw_simple;      /* depthwise: every channel has ls == 0, rs >= 1 */
	/* last inference */
	Kind kind;
	int64_t us;
	int mismatches;   /* verify mode */
	bool fused;       /* did ADD/PRELU in its output step (pie_fusion.h) */
	/* depthwise phase profile, last inference: setup (caller), then
	 * padded-row fill, MAC loop and output step (cycles, per core) */
	int64_t dw_copy_us;
	uint32_t dw_mac_cyc[2], dw_out_cyc[2], dw_fill_cyc[2];
	int dw_oh, dw_ow, dw_stride;
	bool dw_scratch_ext;  /* padded rows landed in PSRAM */
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

size_t flat_size(const TfLiteEvalTensor *t)
{
	size_t n = 1;
	for (int i = 0; i < t->dims->size; i++) n *= t->dims->data[i];
	return n;
}

bool overlaps(const int8_t *a, size_t na, const int8_t *b, size_t nb)
{
	return a < b + nb && b < a + na;
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
/* Fused output step (pie_fusion.h): the conv's int8 result c goes on
 * through the residual ADD and/or the PRELU table before it's stored. */
struct Epi {
	const int8_t *skip;          /* ADD's other input, same layout as the output */
	const int32_t *rs, *rc;      /* ADD rescale tables, indexed by int8 value */
	int32_t om, os, oo, lo, hi;  /* ADD output requantization */
	bool add;
	const int8_t *pt;            /* PRELU table [256][C], or null */
	int C;
};

inline int8_t epi_apply(const Epi *e, int32_t c, size_t idx, int ch)
{
	if (e->add) {
		c = pie_mbqm(e->rs[e->skip[idx]] + e->rc[c], e->om, e->os) + e->oo;
		c = c < e->lo ? e->lo : c > e->hi ? e->hi : c;
	}
	if (e->pt) c = e->pt[(c + 128) * e->C + ch];
	return (int8_t)c;
}

/* x . w[o] for every output channel o of one pixel; out/idx0 = where
 * that pixel's outputs go (element index for the fused epilogue). */
inline void dot_all(const Layer *L, const micro::OpDataConv &d, const int8_t *x, int n16, int8_t *out,
		    const Epi *e, size_t idx0)
{
	const int8_t *w = L->w;
	const int stride = n16 * 16;
	for (int o = 0; o < L->oc; o++, w += stride) {
		int32_t c = requant(dot_s8(x, w, n16) + L->bias[o],
				    d.per_channel_output_multiplier[o], d.per_channel_output_shift[o],
				    d.output_zero_point, d.output_activation_min, d.output_activation_max);
		out[o] = e ? epi_apply(e, c, idx0 + o, o) : (int8_t)c;
	}
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

void pointwise_range(const Layer *L, const micro::OpDataConv &d, const int8_t *in, int8_t *out, int p0, int p1,
		     const Epi *e)
{
	const int ic = L->ic, oc = L->oc, n16 = ic / 16;
	int block = (int)(kWeightBlock / ic);
	if (block < 1) block = 1;
	if (block >= oc) {
		for (int p = p0; p < p1; p++)
			dot_all(L, d, in + (size_t)p * ic, n16, out + (size_t)p * oc, e, (size_t)p * oc);
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
			for (int o = o0; o < o1; o++, w += ic) {
				int32_t c = requant(dot_s8(x, w, n16) + L->bias[o], mult[o], shift[o], out_off, lo, hi);
				y[o] = e ? epi_apply(e, c, (size_t)p * oc + o, o) : (int8_t)c;
			}
		}
	}
}

void conv_pointwise(const Layer *L, const micro::OpDataConv &d, const int8_t *in, int8_t *out, int pixels,
		    const Epi *e)
{
	pie_parallel_for(pixels, [&](int p0, int p1) { pointwise_range(L, d, in, out, p0, p1, e); });
}

bool conv_im2col(const Layer *L, const TfLiteConvParams &p, const micro::OpDataConv &d,
		 const TfLiteEvalTensor *in, TfLiteEvalTensor *out, int8_t *dst_base, const Epi *e)
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
	int8_t *dst = dst_base + (size_t)oy0 * ow * L->oc;
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
			dot_all(L, d, col, L->k16 / 16, dst, e, (size_t)(dst - dst_base));
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

/* Per-channel output parameters for depthwise, precomputed so the output
 * step is branch-free: pie_mbqm() split into a left shift, the rounding
 * high multiply and a rounding right shift (half/nz encode rs == 0). */
struct DwChan {
	int32_t bias;   /* bias + input_offset * sum(w) */
	int32_t mult;
	int32_t half;   /* rs ? 1 << (rs - 1) : 0 */
	uint8_t ls, rs, nz;
};

bool prepare_dw(Layer *L, const TfLiteEvalTensor *filter, const TfLiteEvalTensor *bias,
		const micro::OpDataConv &d)
{
	L->kh = filter->dims->data[1];
	L->kw = filter->dims->data[2];
	L->ic = L->oc = filter->dims->data[3];
	const int C = L->oc, T = L->kh * L->kw;
	/* weights regrouped [g][tap][16]: one 16-channel group's taps are
	 * contiguous, so the MAC loop walks them with a +16 post-increment */
	L->w = (int8_t *)heap_caps_aligned_alloc(16, (size_t)T * C, MALLOC_CAP_SPIRAM);
	L->dwq = (DwChan *)heap_caps_malloc(C * sizeof(DwChan), MALLOC_CAP_SPIRAM);
	if (!L->w || !L->dwq) return false;
	const int8_t *f = filter->data.int8;  /* [tap][c] */
	for (int g = 0; g < C; g += 16)
		for (int t = 0; t < T; t++)
			memcpy(L->w + (size_t)g * T + t * 16, f + (size_t)t * C + g, 16);
	const int32_t input_offset = -d.input_zero_point;
	for (int c = 0; c < C; c++) {
		int32_t s = 0;
		for (int t = 0; t < T; t++) s += f[t * C + c];
		DwChan &q = L->dwq[c];
		q.bias = (bias ? bias->data.i32[c] : 0) + input_offset * s;
		q.mult = d.per_channel_output_multiplier[c];
		const int sh = d.per_channel_output_shift[c];
		q.ls = sh > 0 ? sh : 0;
		q.rs = sh < 0 ? -sh : 0;
		q.nz = q.rs != 0;
		q.half = q.rs ? 1 << (q.rs - 1) : 0;
	}
	L->dw_simple = true;
	for (int c = 0; c < C; c++) L->dw_simple &= L->dwq[c].ls == 0 && L->dwq[c].rs >= 1;
	return true;
}

/* Signed 20-bit field in the low bits of u. */
inline int32_t sx20(uint32_t u) { return (int32_t)(u << 12) >> 12; }

/* Same result as requant(v, mult, shift, ...) with pie_mbqm.
 * SIMPLE: ls == 0 and rs >= 1 for every channel (the usual case), so the
 * rounding needs no masks. */
template <bool SIMPLE>
inline int32_t dw_rq(int32_t v, const DwChan &q)
{
	const int32_t a = SIMPLE ? v : (int32_t)((uint32_t)v << q.ls);
	int32_t h = (int32_t)(((int64_t)a * q.mult + (1 << 30)) >> 31);
	if (SIMPLE) return (h + q.half - (h < 0)) >> q.rs;
	return (h + q.half - (int32_t)(((uint32_t)h >> 31) & q.nz)) >> q.rs;
}

/*
 * Output step for one 16-channel group: QACC as stored by ee.st.qacc_*
 * (lanes 0..7 in words 0..4, lanes 8..15 in words 8..12, 20 bits each),
 * + bias, requantize, clamp, 16 bytes to dst (16-byte aligned).
 * Not inlined, so its own register allocation doesn't compete with the
 * MAC loop's: inlined, the compiler spilled most of it to the stack.
 */
template <bool SIMPLE>
__attribute__((noinline)) void dw_out16(const uint32_t *acc, const DwChan *q, int8_t *dst,
					int32_t off, int32_t lo, int32_t hi)
{
	alignas(4) int8_t o[16];
	for (int half = 0; half < 2; half++, acc += 8, q += 8) {
		const uint32_t w0 = acc[0], w1 = acc[1], w2 = acc[2], w3 = acc[3], w4 = acc[4];
		const int32_t v[8] = {
			sx20(w0), sx20((w0 >> 20) | (w1 << 12)), sx20(w1 >> 8), sx20((w1 >> 28) | (w2 << 4)),
			sx20((w2 >> 16) | (w3 << 16)), sx20(w3 >> 4), sx20((w3 >> 24) | (w4 << 8)), (int32_t)w4 >> 12,
		};
		for (int i = 0; i < 8; i++) {
			int32_t h = dw_rq<SIMPLE>(v[i] + q[i].bias, q[i]) + off;
			h = h < lo ? lo : h;
			h = h > hi ? hi : h;
			o[half * 8 + i] = (int8_t)h;
		}
	}
	uint32_t *d = reinterpret_cast<uint32_t *>(dst);
	const uint32_t *s = reinterpret_cast<const uint32_t *>(o);
	d[0] = s[0]; d[1] = s[1]; d[2] = s[2]; d[3] = s[3];
}

/* Per-core ring of KH padded input rows, internal RAM. */
int8_t *ring_buf[2];
size_t ring_size[2];

int8_t *get_ring(int core, size_t n)
{
	if (n <= ring_size[core]) return ring_buf[core];
	heap_caps_free(ring_buf[core]);
	ring_buf[core] = (int8_t *)heap_caps_aligned_alloc(16, n, MALLOC_CAP_INTERNAL | MALLOC_CAP_8BIT);
	if (!ring_buf[core]) ring_buf[core] = (int8_t *)heap_caps_aligned_alloc(16, n, MALLOC_CAP_SPIRAM);
	ring_size[core] = ring_buf[core] ? n : 0;
	return ring_buf[core];
}

/*
 * Depthwise conv, 16 channels per QACC pass.
 *
 * Input rows are padded into a small per-core ring (KH rows of PW*C
 * bytes, zero point outside the image) as the core walks down its band of
 * output rows, so every tap is in bounds and 16-byte aligned, the padding
 * work is split over both cores, and the window reads hit internal RAM.
 * A 5-wide kernel row is one asm block that loads the next input/weight
 * while multiplying the current pair (EE.VMULAS.S8.QACC.LD.IP).
 */
bool dw_conv(const Layer *L, const TfLiteDepthwiseConvParams &p, const micro::OpDataConv &d,
	     const TfLiteEvalTensor *in, TfLiteEvalTensor *out)
{
	const int ih = in->dims->data[1], iw = in->dims->data[2], C = L->oc;
	const int oh = out->dims->data[1], ow = out->dims->data[2];
	const int sh = p.stride_height, sw = p.stride_width;
	const int ph = d.padding.height, pw = d.padding.width;
	const int kh = L->kh, kw = L->kw, T = kh * kw;
	const int8_t zp = (int8_t)d.input_zero_point;
	const int PW = (ow - 1) * sw + kw;
	const size_t prow = (size_t)PW * C;

	const int64_t tc0 = esp_timer_get_time();
	int8_t *rings[2] = { get_ring(0, kh * prow), get_ring(1, kh * prow) };
	if (!rings[0] || !rings[1]) return false;
	Layer *ML = const_cast<Layer *>(L);
	ML->dw_scratch_ext = esp_ptr_external_ram(rings[0]) || esp_ptr_external_ram(rings[1]);
	ML->dw_oh = oh; ML->dw_ow = ow; ML->dw_stride = sh;
	ML->dw_mac_cyc[0] = ML->dw_mac_cyc[1] = ML->dw_out_cyc[0] = ML->dw_out_cyc[1] = 0;
	ML->dw_fill_cyc[0] = ML->dw_fill_cyc[1] = 0;
	ML->dw_copy_us = esp_timer_get_time() - tc0;

	const int8_t *src = in->data.int8;
	int8_t *dst0 = out->data.int8;
	const int8_t *wts = L->w;
	const DwChan *chan = L->dwq;
	const bool simple = L->dw_simple;
	const int32_t out_off = d.output_zero_point, lo = d.output_activation_min, hi = d.output_activation_max;

	pie_parallel_for(oh, [=](int oy0, int oy1) {
	const int core = xPortGetCoreID();
	int8_t *const ring = rings[core];
	const int step = C;  /* next tap along x, in the padded rows */
	alignas(16) uint32_t acc[16];
	uint32_t mac_cyc = 0, out_cyc = 0, fill_cyc = 0;
	int next = oy0 * sh;  /* next padded row to fill */
	for (int oy = oy0; oy < oy1; oy++) {
		/* bring rows oy*sh .. oy*sh+kh-1 into the ring */
		const uint32_t cf0 = esp_cpu_get_cycle_count();
		for (const int last = oy * sh + kh - 1; next <= last; next++) {
			int8_t *r = ring + (size_t)(next % kh) * prow;
			const int iy = next - ph;
			if (iy < 0 || iy >= ih) { memset(r, zp, prow); continue; }
			const int xa = pw, xb = pw + iw < PW ? pw + iw : PW;
			memset(r, zp, (size_t)xa * C);
			memcpy(r + (size_t)xa * C, src + (size_t)iy * iw * C, (size_t)(xb - xa) * C);
			if (xb < PW) memset(r + (size_t)xb * C, zp, (size_t)(PW - xb) * C);
		}
		fill_cyc += esp_cpu_get_cycle_count() - cf0;
		const int8_t *rows[8];
		for (int ky = 0; ky < kh; ky++) rows[ky] = ring + (size_t)((oy * sh + ky) % kh) * prow;
		int8_t *dst = dst0 + (size_t)oy * ow * C;
		for (int ox = 0; ox < ow; ox++) {
			const size_t xo = (size_t)ox * sw * C;
			for (int g = 0; g < C; g += 16, dst += 16) {
				const uint32_t c0 = esp_cpu_get_cycle_count();
				const int8_t *w = wts + (size_t)g * T;
				asm volatile("ee.zero.qacc");
				for (int ky = 0; ky < kh; ky++) {
					const int8_t *x = rows[ky] + xo + g;
					if (kw == 5) {
						asm volatile(
							"ee.vld.128.xp q0, %0, %2\n\t"
							"ee.vld.128.ip q1, %1, 16\n\t"
							"ee.vld.128.xp q2, %0, %2\n\t"
							"ee.vmulas.s8.qacc.ld.ip q3, %1, 16, q0, q1\n\t"
							"ee.vld.128.xp q0, %0, %2\n\t"
							"ee.vmulas.s8.qacc.ld.ip q1, %1, 16, q2, q3\n\t"
							"ee.vld.128.xp q2, %0, %2\n\t"
							"ee.vmulas.s8.qacc.ld.ip q3, %1, 16, q0, q1\n\t"
							"ee.vld.128.xp q0, %0, %2\n\t"
							"ee.vmulas.s8.qacc.ld.ip q1, %1, 16, q2, q3\n\t"
							"ee.vmulas.s8.qacc q0, q1\n\t"
							: "+a"(x), "+a"(w)
							: "a"(step)
							: "memory");
					} else {
						for (int kx = 0; kx < kw; kx++)
							asm volatile(
								"ee.vld.128.xp q0, %0, %2\n\t"
								"ee.vld.128.ip q1, %1, 16\n\t"
								"ee.vmulas.s8.qacc q0, q1\n\t"
								: "+a"(x), "+a"(w)
								: "a"(step)
								: "memory");
					}
				}
				const uint32_t c1 = esp_cpu_get_cycle_count();
				/* QACC_L = lanes 0..7 -> acc[0..5), QACC_H = lanes 8..15 -> acc[8..13) */
				uint32_t *a = acc;
				asm volatile(
					"ee.st.qacc_l.l.128.ip %0, 16\n\t"
					"ee.st.qacc_l.h.32.ip %0, 16\n\t"
					"ee.st.qacc_h.l.128.ip %0, 16\n\t"
					"ee.st.qacc_h.h.32.ip %0, 0\n\t"
					: "+a"(a) : : "memory");
				if (simple) dw_out16<true>(acc, chan + g, dst, out_off, lo, hi);
				else dw_out16<false>(acc, chan + g, dst, out_off, lo, hi);
				const uint32_t c2 = esp_cpu_get_cycle_count();
				mac_cyc += c1 - c0;
				out_cyc += c2 - c1;
			}
		}
	}
	ML->dw_mac_cyc[core] += mac_cyc;
	ML->dw_out_cyc[core] += out_cyc;
	ML->dw_fill_cyc[core] += fill_cyc;
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

	/* fusion with the ADD/PRELU that consume our output (pie_fusion.h) */
	Epi epi;
	const Epi *ep = nullptr;
	int8_t *dst = out->data.int8;
	PieFused *F = kind != REF ? pie_fusion_get(node->outputs->data[0], true) : nullptr;
	if (F) F->pending = false;
	if (F && pie_fusion_enabled() && (F->add || F->prelu) && (!F->prelu || F->C == L->oc)) {
		TfLiteEvalTensor *fin = ctx->GetEvalTensor(ctx, F->prelu ? F->prelu_out : F->add_out);
		const TfLiteEvalTensor *sk = F->add ? ctx->GetEvalTensor(ctx, F->skip) : nullptr;
		const size_t n = flat_size(out);
		bool safe = fin && flat_size(fin) == n &&
			    !overlaps(fin->data.int8, n, in->data.int8, flat_size(in));
		if (safe && F->add)
			safe = sk && flat_size(sk) == n &&
			       (sk->data.int8 == fin->data.int8 || !overlaps(fin->data.int8, n, sk->data.int8, n));
		if (safe) {
			epi.skip = sk ? sk->data.int8 : nullptr;
			epi.rs = F->r_skip + 128;
			epi.rc = F->r_conv + 128;
			epi.om = F->om; epi.os = F->os; epi.oo = F->oo; epi.lo = F->lo; epi.hi = F->hi;
			epi.add = F->add;
			epi.pt = F->prelu ? F->prelu_t : nullptr;
			epi.C = F->C;
			ep = &epi;
			dst = fin->data.int8;
		}
	}

	bool ok = true;
	if (kind == POINTWISE)
		conv_pointwise(L, data, in->data.int8, dst, out->dims->data[1] * out->dims->data[2], ep);
	else if (kind == IM2COL)
		ok = conv_im2col(L, params, data, in, out, dst, ep);
	if (ok && ep) F->pending = true;
	L->fused = ok && ep;
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
		if (!prepare_dw(L, filter, bias, data)) { heap_caps_free(L->w); L->w = nullptr; heap_caps_free(L->dwq); L->dwq = nullptr; }
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
		printf("%-3d %-5s %-7s %10s %10.2f%s", i, L.dw_op ? "dw" : "conv", kind_name[L.kind], sh, L.us / 1e3,
		       L.fused ? " +fused" : "");
		if (verify && L.kind != REF) printf(" %d", L.mismatches);
		printf("\n");
	}
	for (int k = 0; k < 4; k++)
		if (count[k]) printf("%-9s %2d layers %10.1f ms\n", kind_name[k], count[k], us[k] / 1e3);
	/* depthwise phases: cycles per output value, summed over both cores */
	printf("\ndepthwise phases (cycles per output, both cores summed):\n");
	printf("%-3s %-14s %6s %8s %8s %8s %8s %s\n", "#", "out", "stride", "ms", "fill c/o", "MAC c/o", "out c/o", "rows in");
	double t_fill = 0, t_mac = 0, t_out = 0, n_out = 0;
	for (int i = 0; i < n_layers; i++) {
		const Layer &L = layers[i];
		if (L.kind != DEPTHWISE) continue;
		const double n = (double)L.dw_oh * L.dw_ow * L.oc;
		const double mac = (double)L.dw_mac_cyc[0] + L.dw_mac_cyc[1], o = (double)L.dw_out_cyc[0] + L.dw_out_cyc[1];
		const double fl = (double)L.dw_fill_cyc[0] + L.dw_fill_cyc[1];
		char sh[24];
		snprintf(sh, sizeof(sh), "%dx%dx%d", L.dw_oh, L.dw_ow, L.oc);
		printf("%-3d %-14s %6d %8.2f %8.1f %8.1f %8.1f %s\n", i, sh, L.dw_stride, L.us / 1e3, fl / n,
		       mac / n, o / n, L.dw_scratch_ext ? "PSRAM" : "internal");
		t_fill += fl; t_mac += mac; t_out += o; n_out += n;
	}
	if (n_out > 0)
		printf("all (core time, both cores): row fill %.1f c/o %.1f ms, MAC %.1f c/o %.1f ms, output step %.1f c/o %.1f ms\n",
		       t_fill / n_out, t_fill / 240e3, t_mac / n_out, t_mac / 240e3, t_out / n_out, t_out / 240e3);
	if (verify) printf("%s\n", bad ? "VERIFY MISMATCH" : "verify: all PIE layers bit-exact");
	pie_elementwise_print_stats();
	pie_data_print_stats();
	pie_parallel_print_stats();
	pie_fusion_print();
}
