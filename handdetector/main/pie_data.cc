/*
 * Data-movement ops -- MAX_POOL_2D, PAD, RESIZE_BILINEAR -- for int8.
 *
 * None of these do arithmetic on the quantized values that needs
 * requantizing (onnx2tf leaves their input and output qparams equal, see
 * convert.py's unify pass), so they are pure memory work. TFLM's
 * reference versions are generic N-d loops calling Offset() per element;
 * here each is a straight pass over rows, split over both cores.
 *
 *  MAX_POOL   VALID windows (the palm model: 2x2 stride 2). With 16-byte
 *             aligned rows of a multiple of 16 channels, each window is a
 *             few EE.VMAX.S8 over 16 channels at a time; otherwise scalar.
 *             Same activation clamp as the reference.
 *  PAD        4-d NHWC with batch padding 0: whole padded rows are one
 *             memset, interior pixels are memset + memcpy + memset (the
 *             model pads either H/W, before stride-2 depthwise convs, or
 *             channels, on the residual skip path). Pad value as TFLM:
 *             the constant input if present, else the output zero point.
 *  RESIZE     TFLM's integer bilinear formula (ResizeBilinearInteger)
 *             with the per-row and per-column source indices and 10-bit
 *             weights computed once, the per-channel sum in 32 bits (it
 *             can't overflow: |v| <= 128 and the weights' absolute sum is
 *             < 1536^2) and the same symmetric rounding (double-rounding
 *             build, TFLITE_SINGLE_ROUNDING undefined).
 *
 * Bit-exact with the reference: `det pie verify on` reruns it per layer.
 */
#include <stdio.h>
#include <string.h>

#include "esp_heap_caps.h"
#include "esp_timer.h"

#include "tensorflow/lite/micro/kernels/kernel_util.h"
#include "tensorflow/lite/micro/kernels/micro_ops.h"
#include "tensorflow/lite/micro/kernels/pad.h"
#include "tensorflow/lite/micro/kernels/pooling.h"

#include "pie_kernels.h"
#include "pie_parallel.h"

namespace {

enum Op { MAX_POOL, PAD, RESIZE, N_OPS };
const char *const op_name[] = { "MAX_POOL", "PAD", "RESIZE" };
TFLMRegistration ref[N_OPS];

struct Stats { int layers, pie, mismatches; int64_t us; } stats[N_OPS];

size_t flat(const TfLiteEvalTensor *t)
{
	size_t n = 1;
	for (int i = 0; i < t->dims->size; i++) n *= t->dims->data[i];
	return n;
}

inline bool aligned16(const void *p) { return ((uintptr_t)p & 15) == 0; }

/* ---- MAX_POOL_2D ---- */

bool eval_maxpool(TfLiteContext *ctx, TfLiteNode *node)
{
	const auto *params = static_cast<const TfLitePoolParams *>(node->builtin_data);
	const auto *d = static_cast<const tflite::micro::OpDataPooling *>(node->user_data);
	const TfLiteEvalTensor *in = tflite::micro::GetEvalInput(ctx, node, 0);
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (in->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (in->dims->size != 4 || out->dims->size != 4 || in->dims->data[0] != 1) return false;
	if (d->padding.height || d->padding.width) return false;
	const int H = in->dims->data[1], W = in->dims->data[2], C = in->dims->data[3];
	const int OH = out->dims->data[1], OW = out->dims->data[2];
	const int fh = params->filter_height, fw = params->filter_width;
	const int sh = params->stride_height, sw = params->stride_width;
	if (out->dims->data[3] != C || (OH - 1) * sh + fh > H || (OW - 1) * sw + fw > W) return false;

	const int8_t lo = (int8_t)d->activation_min, hi = (int8_t)d->activation_max;
	const int8_t *x = in->data.int8;
	int8_t *y = out->data.int8;
	const bool simd = C % 16 == 0 && aligned16(x) && aligned16(y) && lo == -128 && hi == 127;
	pie_parallel_for(OH, [&](int r0, int r1) {
		for (int oy = r0; oy < r1; oy++) {
			for (int ox = 0; ox < OW; ox++) {
				const int8_t *p = x + ((size_t)(oy * sh) * W + ox * sw) * C;
				int8_t *q = y + ((size_t)oy * OW + ox) * C;
				if (simd && fh == 2 && fw == 2) {
					for (int c = 0; c < C; c += 16) {
						const int8_t *a = p + c, *b = p + C + c;
						const int8_t *e = p + (size_t)W * C + c, *f = e + C;
						int8_t *o = q + c;
						asm volatile(
							"ee.vld.128.ip q0, %0, 0\n"
							"ee.vld.128.ip q1, %1, 0\n"
							"ee.vmax.s8 q0, q0, q1\n"
							"ee.vld.128.ip q1, %2, 0\n"
							"ee.vmax.s8 q0, q0, q1\n"
							"ee.vld.128.ip q1, %3, 0\n"
							"ee.vmax.s8 q0, q0, q1\n"
							"ee.vst.128.ip q0, %4, 0\n"
							: "+a"(a), "+a"(b), "+a"(e), "+a"(f), "+a"(o) :: "memory");
					}
				} else if (simd) {
					for (int c = 0; c < C; c += 16) {
						const int8_t *a = p + c;
						asm volatile("ee.vld.128.ip q0, %0, 0" : "+a"(a) :: "memory");
						for (int ky = 0; ky < fh; ky++)
							for (int kx = 0; kx < fw; kx++) {
								const int8_t *b = p + ((size_t)ky * W + kx) * C + c;
								asm volatile(
									"ee.vld.128.ip q1, %0, 0\n"
									"ee.vmax.s8 q0, q0, q1\n"
									: "+a"(b) :: "memory");
							}
						int8_t *o = q + c;
						asm volatile("ee.vst.128.ip q0, %0, 0" : "+a"(o) :: "memory");
					}
				} else {
					for (int c = 0; c < C; c++) {
						int8_t m = -128;
						for (int ky = 0; ky < fh; ky++)
							for (int kx = 0; kx < fw; kx++) {
								const int8_t v = p[((size_t)ky * W + kx) * C + c];
								if (v > m) m = v;
							}
						q[c] = m < lo ? lo : m > hi ? hi : m;
					}
				}
			}
		}
	});
	return true;
}

/* ---- PAD ---- */

bool eval_pad(TfLiteContext *ctx, TfLiteNode *node)
{
	const auto *d = static_cast<const tflite::micro::OpData *>(node->user_data);
	const TfLiteEvalTensor *in = tflite::micro::GetEvalInput(ctx, node, 0);
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (in->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (in->dims->size != 4 || out->dims->size != 4 || in->dims->data[0] != 1) return false;
	const tflite::PadParams &pp = d->params;
	if (pp.left_padding_count != 4 || pp.right_padding_count != 4) return false;
	if (pp.left_padding[0] || pp.right_padding[0]) return false;
	for (int i = 1; i < 4; i++)
		if (pp.left_padding[i] < 0 || pp.right_padding[i] < 0 ||
		    in->dims->data[i] + pp.left_padding[i] + pp.right_padding[i] != out->dims->data[i])
			return false;
	int8_t pv = (int8_t)(uint8_t)d->output_zero_point;
	if (node->inputs->size == 3) {
		const TfLiteEvalTensor *cv = tflite::micro::GetEvalInput(ctx, node, 2);
		if (cv) pv = *cv->data.int8;
	}
	const int H = in->dims->data[1], W = in->dims->data[2], C = in->dims->data[3];
	const int OH = out->dims->data[1], OW = out->dims->data[2], OC = out->dims->data[3];
	const int ph = pp.left_padding[1], pw = pp.left_padding[2], pc = pp.left_padding[3];
	const int8_t *x = in->data.int8;
	int8_t *y = out->data.int8;
	pie_parallel_for(OH, [&](int r0, int r1) {
		for (int oy = r0; oy < r1; oy++) {
			int8_t *row = y + (size_t)oy * OW * OC;
			const int iy = oy - ph;
			if (iy < 0 || iy >= H) {
				memset(row, pv, (size_t)OW * OC);
				continue;
			}
			const int8_t *src = x + (size_t)iy * W * C;
			memset(row, pv, (size_t)pw * OC);
			int8_t *dst = row + (size_t)pw * OC;
			if (OC == C) {
				memcpy(dst, src, (size_t)W * C);
			} else {
				for (int ix = 0; ix < W; ix++, dst += OC, src += C) {
					if (pc) memset(dst, pv, pc);
					memcpy(dst + pc, src, C);
					if (OC - C - pc) memset(dst + pc + C, pv, OC - C - pc);
				}
			}
			memset(row + (size_t)(pw + W) * OC, pv, (size_t)(OW - pw - W) * OC);
		}
	});
	return true;
}

/* ---- RESIZE_BILINEAR ---- */

/* TFLM's ComputeInterpolationValuesInteger, plus the two 10-bit weights */
struct Tap { int i0, i1; int32_t w0, w1; };

Tap tap(int v, int32_t scale_10, bool half_pixel, int size)
{
	const int32_t s = half_pixel ? v * scale_10 + scale_10 / 2 - (1 << 9) : v * scale_10;
	Tap t;
	t.i0 = s / (1 << 10) > 0 ? s / (1 << 10) : 0;
	t.i1 = (s + (1 << 10) - 1) / (1 << 10);
	if (t.i1 > size - 1) t.i1 = size - 1;
	t.w1 = s - (1 << 10) * t.i0;
	t.w0 = (1 << 10) - t.w1;
	return t;
}

bool eval_resize(TfLiteContext *ctx, TfLiteNode *node)
{
	const auto *params = static_cast<const TfLiteResizeBilinearParams *>(node->builtin_data);
	const TfLiteEvalTensor *in = tflite::micro::GetEvalInput(ctx, node, 0);
	const TfLiteEvalTensor *size = tflite::micro::GetEvalInput(ctx, node, 1);
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (in->type != kTfLiteInt8 || out->type != kTfLiteInt8 || size->type != kTfLiteInt32) return false;
	if (in->dims->size != 4 || out->dims->size != 4 || in->dims->data[0] != 1 || flat(size) != 2) return false;
	if (params->align_corners) return false;
	const int H = in->dims->data[1], W = in->dims->data[2], C = in->dims->data[3];
	const int OH = size->data.i32[0], OW = size->data.i32[1];
	if (OH != out->dims->data[1] || OW != out->dims->data[2] || out->dims->data[3] != C) return false;
	constexpr int kMax = 128;
	if (OH > kMax || OW > kMax) return false;

	const int32_t hs = ((1 << 10) * H + OH / 2) / OH;
	const int32_t ws = ((1 << 10) * W + OW / 2) / OW;
	/* 4 KB: static, not on the caller's stack (the console task runs
	 * `det golden`); only one inference runs at a time */
	static Tap ty[kMax], tx[kMax];
	for (int i = 0; i < OH; i++) ty[i] = tap(i, hs, params->half_pixel_centers, H);
	for (int i = 0; i < OW; i++) tx[i] = tap(i, ws, params->half_pixel_centers, W);

	const int8_t *x = in->data.int8;
	int8_t *y = out->data.int8;
	pie_parallel_for(OH, [&](int r0, int r1) {
		for (int oy = r0; oy < r1; oy++) {
			const Tap &a = ty[oy];
			const int8_t *row0 = x + (size_t)a.i0 * W * C, *row1 = x + (size_t)a.i1 * W * C;
			int8_t *q = y + (size_t)oy * OW * C;
			for (int ox = 0; ox < OW; ox++, q += C) {
				const Tap &b = tx[ox];
				const int8_t *p00 = row0 + (size_t)b.i0 * C, *p01 = row0 + (size_t)b.i1 * C;
				const int8_t *p10 = row1 + (size_t)b.i0 * C, *p11 = row1 + (size_t)b.i1 * C;
				const int32_t w00 = a.w0 * b.w0, w01 = a.w0 * b.w1;
				const int32_t w10 = a.w1 * b.w0, w11 = a.w1 * b.w1;
				for (int c = 0; c < C; c++) {
					const int32_t s = p00[c] * w00 + p10[c] * w10 + p01[c] * w01 + p11[c] * w11;
					q[c] = (int8_t)((s + (s > 0 ? (1 << 19) : -(1 << 19))) / (1 << 20));
				}
			}
		}
	});
	return true;
}

/* ---- dispatch ---- */

template <Op op>
TfLiteStatus Eval(TfLiteContext *ctx, TfLiteNode *node)
{
	Stats &st = stats[op];
	int64_t t0 = esp_timer_get_time();
	bool ok = false;
	if (pie_enabled()) {
		switch (op) {
		case MAX_POOL: ok = eval_maxpool(ctx, node); break;
		case PAD: ok = eval_pad(ctx, node); break;
		case RESIZE: ok = eval_resize(ctx, node); break;
		default: break;
		}
	}
	TfLiteStatus s = kTfLiteOk;
	if (!ok) s = ref[op].invoke(ctx, node);
	st.us += esp_timer_get_time() - t0;
	st.layers++;
	if (ok) {
		st.pie++;
		if (pie_verify_enabled()) {
			TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
			size_t n = flat(out);
			int8_t *mine = (int8_t *)heap_caps_malloc(n, MALLOC_CAP_SPIRAM);
			if (mine) {
				memcpy(mine, out->data.int8, n);
				ref[op].invoke(ctx, node);
				for (size_t i = 0; i < n; i++) st.mismatches += mine[i] != out->data.int8[i];
				heap_caps_free(mine);
			}
		}
	}
	return s;
}

template <Op op>
TFLMRegistration wrap(TFLMRegistration r)
{
	ref[op] = r;
	r.invoke = Eval<op>;
	return r;
}

}  // namespace

TFLMRegistration PieRegister_MAX_POOL_2D() { return wrap<MAX_POOL>(tflite::Register_MAX_POOL_2D()); }
TFLMRegistration PieRegister_PAD() { return wrap<PAD>(tflite::Register_PAD()); }
TFLMRegistration PieRegister_RESIZE_BILINEAR() { return wrap<RESIZE>(tflite::Register_RESIZE_BILINEAR()); }

void pie_data_reset_stats(void)
{
	memset(stats, 0, sizeof(stats));
}

void pie_data_print_stats(void)
{
	int bad = 0;
	for (int op = 0; op < N_OPS; op++) {
		const Stats &s = stats[op];
		if (!s.layers) continue;
		printf("%-9s %2d layers %10.1f ms  (%d on own path)", op_name[op], s.layers, s.us / 1e3, s.pie);
		if (pie_verify_enabled()) printf("  %d mismatches", s.mismatches);
		printf("\n");
		bad += s.mismatches;
	}
	if (pie_verify_enabled()) printf("%s\n", bad ? "DATA-OP VERIFY MISMATCH" : "verify: all data-movement layers bit-exact");
}
