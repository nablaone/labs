/*
 * Elementwise int8 ops -- ABS, RELU, ADD, SUB, MUL -- with lookup tables.
 *
 * onnx2tf's tf_converter path writes every PRelu as ABS/SUB/MUL/RELU/ADD
 * (findings in docs/model-pipeline.md), so these run ~180 times per
 * inference. TFLM's reference versions call a function pointer per
 * element and requantize up to three times (64-bit fixed-point each).
 * An int8 input has only 256 values, so most of that is a table lookup:
 *
 *  ABS, RELU  out = T[x]. The 256-entry table is produced by running
 *             TFLM's own kernel once on the 256 possible inputs, so it is
 *             exact by construction.
 *  ADD, SUB   reference: requant(r1(a) +/- r2(b)), where r1/r2 rescale each
 *             input to a common scale. r1 and r2 become 256-entry int32
 *             tables; only the final requantization stays per element.
 *  MUL        here the second input is a per-channel constant (the PRelu
 *             slope). out = requant((a + off1) * (b_c + off2)): one 256-entry
 *             table per channel when channels are long (H*W >= 64), the
 *             same formula inline otherwise.
 *  PRELU      native op (models converted with onnx2tf's own writer):
 *             out = T[x][c] with a per-channel slope along the last axis.
 *             The [256][C] table is produced by TFLM's own kernel on a fake
 *             256 x C input (row v = value v in every channel), so it is
 *             exact by construction; rebuilt only if the slopes change.
 *
 * All arithmetic uses TFLM's own MultiplyByQuantizedMultiplier* helpers,
 * so results are bit-exact (`det pie verify on` checks every layer).
 * Anything else (broadcast ADD/SUB, other types) goes to the reference.
 */
#include <stdio.h>
#include <string.h>

#include "esp_heap_caps.h"
#include "esp_timer.h"

#include "tensorflow/lite/micro/kernels/activations.h"
#include "tensorflow/lite/micro/kernels/add.h"
#include "tensorflow/lite/micro/kernels/kernel_util.h"
#include "tensorflow/lite/micro/kernels/micro_ops.h"
#include "tensorflow/lite/micro/kernels/mul.h"
#include "tensorflow/lite/micro/kernels/sub.h"
#include "tensorflow/lite/micro/kernels/internal/common.h"
#include "tensorflow/lite/micro/kernels/internal/types.h"

#include "pie_kernels.h"
#include "pie_parallel.h"
#include "pie_fusion.h"

namespace {

using tflite::micro::MultiplyByQuantizedMultiplier;
using tflite::micro::MultiplyByQuantizedMultiplierSmallerThanOneExp;

enum Op { ABS, RELU, ADD, SUB, MUL, PRELU, N_OPS };
const char *const op_name[] = { "ABS", "RELU", "ADD", "SUB", "MUL", "PRELU" };
TFLMRegistration ref[N_OPS];

struct Layer {
	const TfLiteNode *node;
	bool ready;
	int8_t lut[256];          /* ABS, RELU */
	int8_t *b_copy;           /* MUL, PRELU: the constant input it was built for */
	int8_t *mul_lut;          /* MUL: [C][256]; PRELU: [256][C] */
	int channels;
};
constexpr int kMaxLayers = 256;
Layer *layers;  /* kMaxLayers, in PSRAM: 70 KB that internal RAM needs more */
int n_layers;

struct Stats { int layers, pie, fused, mismatches; int64_t us; } stats[N_OPS];

Layer *find_layer(const TfLiteNode *node)
{
	if (!layers) {
		layers = (Layer *)heap_caps_calloc(kMaxLayers, sizeof(Layer), MALLOC_CAP_SPIRAM);
		if (!layers) return nullptr;
	}
	for (int i = 0; i < n_layers; i++)
		if (layers[i].node == node) return &layers[i];
	if (n_layers == kMaxLayers) return nullptr;
	layers[n_layers] = Layer{};
	layers[n_layers].node = node;
	return &layers[n_layers++];
}

inline int clamp(int32_t v, int32_t lo, int32_t hi) { return v < lo ? lo : v > hi ? hi : v; }

size_t flat(const TfLiteEvalTensor *t)
{
	size_t n = 1;
	for (int i = 0; i < t->dims->size; i++) n *= t->dims->data[i];
	return n;
}

bool same_dims(const TfLiteEvalTensor *a, const TfLiteEvalTensor *b)
{
	if (a->dims->size != b->dims->size) return false;
	for (int i = 0; i < a->dims->size; i++)
		if (a->dims->data[i] != b->dims->data[i]) return false;
	return true;
}

/* ---- ABS, RELU: let the reference kernel fill the table ---- */

bool build_unary_lut(Op op, Layer *L, TfLiteContext *ctx, TfLiteNode *node,
		     TfLiteEvalTensor *in, TfLiteEvalTensor *out)
{
	static int8_t all[256];
	for (int i = 0; i < 256; i++) all[i] = (int8_t)(i - 128);
	struct { int size; int data[1]; } dims = { 1, { 256 } };
	TfLitePtrUnion in_data = in->data, out_data = out->data;
	TfLiteIntArray *in_dims = in->dims, *out_dims = out->dims;
	in->data.int8 = all;
	out->data.int8 = L->lut;
	in->dims = out->dims = reinterpret_cast<TfLiteIntArray *>(&dims);
	TfLiteStatus s = ref[op].invoke(ctx, node);
	in->data = in_data; out->data = out_data;
	in->dims = in_dims; out->dims = out_dims;
	return s == kTfLiteOk;
}

bool eval_unary(Op op, Layer *L, TfLiteContext *ctx, TfLiteNode *node)
{
	auto *in = const_cast<TfLiteEvalTensor *>(tflite::micro::GetEvalInput(ctx, node, 0));
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (in->type != kTfLiteInt8 || out->type != kTfLiteInt8 || flat(in) != flat(out)) return false;
	if (!L->ready) {
		if (!build_unary_lut(op, L, ctx, node, in, out)) return false;
		L->ready = true;
	}
	const int8_t *x = in->data.int8;
	int8_t *y = out->data.int8;
	const int8_t *t = L->lut + 128;
	pie_parallel_for((int)flat(out), [&](int b, int e) {
		for (int i = b; i < e; i++) y[i] = t[x[i]];
	});
	return true;
}

/* ---- ADD, SUB ---- */

template <typename OpData>
bool eval_addsub(bool sub, TfLiteContext *ctx, TfLiteNode *node)
{
	const auto &d = *static_cast<const OpData *>(node->user_data);
	const TfLiteEvalTensor *a = tflite::micro::GetEvalInput(ctx, node, 0);
	const TfLiteEvalTensor *b = tflite::micro::GetEvalInput(ctx, node, 1);
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (a->type != kTfLiteInt8 || b->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (!same_dims(a, out) || !same_dims(b, out)) return false;

	/* per-input rescale, exactly as the reference does it per element */
	int32_t t1[256], t2[256];
	for (int v = -128; v < 128; v++) {
		t1[v + 128] = MultiplyByQuantizedMultiplierSmallerThanOneExp(
			(d.input1_offset + v) * (1 << d.left_shift), d.input1_multiplier, d.input1_shift);
		t2[v + 128] = MultiplyByQuantizedMultiplierSmallerThanOneExp(
			(d.input2_offset + v) * (1 << d.left_shift), d.input2_multiplier, d.input2_shift);
	}
	const int32_t *r1 = t1 + 128, *r2 = t2 + 128;
	const int32_t om = d.output_multiplier, os = d.output_shift, oo = d.output_offset;
	const int32_t lo = d.output_activation_min, hi = d.output_activation_max;

	/* fusion: if one input is a conv output read only by this ADD, give
	 * that conv what it needs to do this ADD itself (see pie_fusion.h) */
	for (int k = 0; !sub && k < 2; k++) {
		const int in_t = node->inputs->data[k];
		PieFused *F = pie_fusion_get(in_t, false);
		if (!F || F->add || pie_fusion_consumers(in_t) != 1) continue;
		memcpy(F->r_conv, k == 0 ? t1 : t2, sizeof(t1));
		memcpy(F->r_skip, k == 0 ? t2 : t1, sizeof(t1));
		F->skip = node->inputs->data[1 - k];
		F->om = om; F->os = os; F->oo = oo; F->lo = lo; F->hi = hi;
		F->add_out = node->outputs->data[0];
		F->add = true;
		break;
	}
	const int8_t *x = a->data.int8, *z = b->data.int8;
	int8_t *y = out->data.int8;
	const size_t n = flat(out);
	pie_parallel_for((int)n, [&](int b, int e) {
		if (sub) {
			for (int i = b; i < e; i++)
				y[i] = (int8_t)clamp(pie_mbqm(r1[x[i]] - r2[z[i]], om, os) + oo, lo, hi);
		} else {
			for (int i = b; i < e; i++)
				y[i] = (int8_t)clamp(pie_mbqm(r1[x[i]] + r2[z[i]], om, os) + oo, lo, hi);
		}
	});
	return true;
}

/* ---- MUL by a per-channel constant ---- */

bool eval_mul(Layer *L, TfLiteContext *ctx, TfLiteNode *node)
{
	const auto &d = *static_cast<const tflite::micro::OpDataMul *>(node->user_data);
	const TfLiteEvalTensor *a = tflite::micro::GetEvalInput(ctx, node, 0);
	const TfLiteEvalTensor *b = tflite::micro::GetEvalInput(ctx, node, 1);
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (a->type != kTfLiteInt8 || b->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (!same_dims(a, out)) return false;

	/* b broadcasts along exactly one axis k of the output: right-align
	 * its dims, every other dim must be 1. */
	const int r = out->dims->size, rb = b->dims->size;
	if (rb > r) return false;
	int k = -1;
	for (int i = 0; i < rb; i++) {
		int bd = b->dims->data[i], od = out->dims->data[r - rb + i];
		if (bd == 1) continue;
		if (bd != od || k >= 0) return false;
		k = r - rb + i;
	}
	if (k < 0) return false;
	size_t outer = 1, inner = 1;
	for (int i = 0; i < k; i++) outer *= out->dims->data[i];
	for (int i = k + 1; i < r; i++) inner *= out->dims->data[i];
	const int C = out->dims->data[k];

	const int32_t off1 = -d.input1_zero_point, off2 = -d.input2_zero_point;
	const int32_t om = d.output_multiplier, os = d.output_shift, oo = d.output_zero_point;
	const int32_t lo = d.output_activation_min, hi = d.output_activation_max;
	const int8_t *x = a->data.int8, *bc = b->data.int8;
	int8_t *y = out->data.int8;

	/* rows: outer * C runs of `inner` elements, each of one channel */
	const int rows = (int)(outer * C);
	if (inner < 64) {
		pie_parallel_for(rows, [&](int r0, int r1) {
			for (int r = r0; r < r1; r++) {
				const int32_t bv = off2 + bc[r % C];
				const int8_t *xs = x + (size_t)r * inner;
				int8_t *ys = y + (size_t)r * inner;
				for (size_t i = 0; i < inner; i++)
					ys[i] = (int8_t)clamp(oo + pie_mbqm((off1 + xs[i]) * bv, om, os), lo, hi);
			}
		});
		return true;
	}

	/* long channels: one table per channel, rebuilt only if b changes */
	if (!L->mul_lut || L->channels != C || memcmp(L->b_copy, bc, C) != 0) {
		heap_caps_free(L->mul_lut);
		heap_caps_free(L->b_copy);
		L->mul_lut = (int8_t *)heap_caps_malloc((size_t)C * 256, MALLOC_CAP_SPIRAM);
		L->b_copy = (int8_t *)heap_caps_malloc(C, MALLOC_CAP_SPIRAM);
		if (!L->mul_lut || !L->b_copy) {
			heap_caps_free(L->mul_lut); heap_caps_free(L->b_copy);
			L->mul_lut = L->b_copy = nullptr;
			return false;
		}
		memcpy(L->b_copy, bc, C);
		L->channels = C;
		for (int c = 0; c < C; c++)
			for (int v = -128; v < 128; v++)
				L->mul_lut[c * 256 + v + 128] = (int8_t)clamp(
					oo + MultiplyByQuantizedMultiplier((off1 + v) * (off2 + bc[c]), om, os), lo, hi);
	}
	pie_parallel_for(rows, [&](int r0, int r1) {
		int8_t row[256];  /* current channel's table, on this core's stack */
		for (int r = r0; r < r1; r++) {
			memcpy(row, L->mul_lut + (r % C) * 256, 256);
			const int8_t *t = row + 128;
			const int8_t *xs = x + (size_t)r * inner;
			int8_t *ys = y + (size_t)r * inner;
			for (size_t i = 0; i < inner; i++) ys[i] = t[xs[i]];
		}
	});
	return true;
}

/* ---- PRELU with a per-channel slope on the last axis ---- */

bool eval_prelu(Layer *L, TfLiteContext *ctx, TfLiteNode *node)
{
	auto *in = const_cast<TfLiteEvalTensor *>(tflite::micro::GetEvalInput(ctx, node, 0));
	const TfLiteEvalTensor *alpha = tflite::micro::GetEvalInput(ctx, node, 1);
	TfLiteEvalTensor *out = tflite::micro::GetEvalOutput(ctx, node, 0);
	if (in->type != kTfLiteInt8 || alpha->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (!same_dims(in, out) || in->dims->size != 4) return false;
	const int C = in->dims->data[3];
	if ((int)flat(alpha) != C || alpha->dims->data[alpha->dims->size - 1] != C) return false;

	if (!L->mul_lut || L->channels != C || memcmp(L->b_copy, alpha->data.int8, C) != 0) {
		heap_caps_free(L->mul_lut);
		heap_caps_free(L->b_copy);
		L->mul_lut = (int8_t *)heap_caps_malloc((size_t)256 * C, MALLOC_CAP_SPIRAM);
		L->b_copy = (int8_t *)heap_caps_malloc(C, MALLOC_CAP_SPIRAM);
		int8_t *probe = (int8_t *)heap_caps_malloc((size_t)256 * C, MALLOC_CAP_SPIRAM);
		bool ok = L->mul_lut && L->b_copy && probe;
		if (ok) {
			for (int v = 0; v < 256; v++) memset(probe + v * C, v - 128, C);
			struct { int size; int data[4]; } dims = { 4, { 1, 1, 256, C } };
			TfLitePtrUnion in_data = in->data, out_data = out->data;
			TfLiteIntArray *in_dims = in->dims, *out_dims = out->dims;
			in->data.int8 = probe;
			out->data.int8 = L->mul_lut;
			in->dims = out->dims = reinterpret_cast<TfLiteIntArray *>(&dims);
			ok = ref[PRELU].invoke(ctx, node) == kTfLiteOk;
			in->data = in_data; out->data = out_data;
			in->dims = in_dims; out->dims = out_dims;
		}
		heap_caps_free(probe);
		if (!ok) {
			heap_caps_free(L->mul_lut); heap_caps_free(L->b_copy);
			L->mul_lut = L->b_copy = nullptr;
			return false;
		}
		memcpy(L->b_copy, alpha->data.int8, C);
		L->channels = C;
	}
	/* fusion: input read only by this PRELU and produced by a conv (or by
	 * an ADD already fused into one) -> the conv can apply the table */
	{
		const int in_t = node->inputs->data[0];
		PieFused *F = pie_fusion_by_add_out(in_t);
		if (!F) {
			F = pie_fusion_get(in_t, false);
			if (F && F->add) F = nullptr;
		}
		if (F && !F->prelu && pie_fusion_consumers(in_t) == 1) {
			const auto &pp = *static_cast<const tflite::micro::PreluParams *>(node->user_data);
			heap_caps_free(F->p_alpha);
			F->p_alpha = (int32_t *)heap_caps_malloc(C * sizeof(int32_t), MALLOC_CAP_SPIRAM);
			if (F->p_alpha) {
				for (int c = 0; c < C; c++) F->p_alpha[c] = pp.alpha_offset + alpha->data.int8[c];
				F->p_in_off = pp.input_offset;
				F->p_out_off = pp.output_offset;
				F->p_m1 = pp.output_multiplier_1; F->p_s1 = pp.output_shift_1;
				F->p_m2 = pp.output_multiplier_2; F->p_s2 = pp.output_shift_2;
				F->prelu_out = node->outputs->data[0];
				F->C = C;
				F->prelu = true;
			}
		}
	}
	const int8_t *x = in->data.int8;
	int8_t *y = out->data.int8;
	const int8_t *t = L->mul_lut + 128 * C;  /* row for x = 0 */
	pie_parallel_for((int)(flat(out) / C), [&](int p0, int p1) {
		const int8_t *xs = x + (size_t)p0 * C;
		int8_t *ys = y + (size_t)p0 * C;
		for (int p = p0; p < p1; p++, xs += C, ys += C)
			for (int c = 0; c < C; c++) ys[c] = t[xs[c] * C + c];
	});
	return true;
}

/* ---- dispatch ---- */

template <Op op>
TfLiteStatus Eval(TfLiteContext *ctx, TfLiteNode *node)
{
	Stats &st = stats[op];
	/* fused into the conv that produced our input: already done */
	if (op == ADD || op == PRELU) {
		PieFused *F = nullptr;
		if (op == ADD) {
			F = pie_fusion_by_add_out(node->outputs->data[0]);
		} else {
			F = pie_fusion_by_add_out(node->inputs->data[0]);
			if (!F) F = pie_fusion_get(node->inputs->data[0], false);
			if (F && !F->prelu) F = nullptr;
		}
		if (F && F->pending) {
			if (op == PRELU || !F->prelu) F->pending = false;
			st.layers++;
			st.fused++;
			return kTfLiteOk;
		}
	}
	Layer *L = find_layer(node);
	int64_t t0 = esp_timer_get_time();
	bool ok = false;
	if (pie_enabled() && L) {
		switch (op) {
		case ABS: case RELU: ok = eval_unary(op, L, ctx, node); break;
		case ADD: ok = eval_addsub<tflite::micro::OpDataAdd>(false, ctx, node); break;
		case SUB: ok = eval_addsub<tflite::micro::OpDataSub>(true, ctx, node); break;
		case MUL: ok = eval_mul(L, ctx, node); break;
		case PRELU: ok = eval_prelu(L, ctx, node); break;
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

TFLMRegistration PieRegister_ABS() { return wrap<ABS>(tflite::micro::Register_ABS()); }
TFLMRegistration PieRegister_RELU() { return wrap<RELU>(tflite::micro::Register_RELU()); }
TFLMRegistration PieRegister_ADD() { return wrap<ADD>(tflite::micro::Register_ADD()); }
TFLMRegistration PieRegister_SUB() { return wrap<SUB>(tflite::micro::Register_SUB()); }
TFLMRegistration PieRegister_MUL() { return wrap<MUL>(tflite::micro::Register_MUL()); }
TFLMRegistration PieRegister_PRELU() { return wrap<PRELU>(tflite::micro::Register_PRELU()); }

void pie_elementwise_reset_stats(void)
{
	memset(stats, 0, sizeof(stats));
}

void pie_elementwise_print_stats(void)
{
	int bad = 0;
	for (int op = 0; op < N_OPS; op++) {
		const Stats &s = stats[op];
		if (!s.layers) continue;
		printf("%-9s %2d layers %10.1f ms  (%d on LUT path, %d fused into conv)", op_name[op], s.layers, s.us / 1e3, s.pie, s.fused);
		if (pie_verify_enabled()) printf("  %d mismatches", s.mismatches);
		printf("\n");
		bad += s.mismatches;
	}
	if (pie_verify_enabled()) printf("%s\n", bad ? "ELEMENTWISE VERIFY MISMATCH" : "verify: all LUT layers bit-exact");
}
