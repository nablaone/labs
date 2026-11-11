/*
 * CONV_2D with a PIE fast path for pointwise (1x1) convolutions -- 74% of
 * the palm detector's MACs (29 layers, input channels 32..256).
 *
 * NHWC int8, filter [OC][1][1][IC]: every output value is a dot product of
 * one input pixel's IC channels with one filter row. EE.VMULAS.S8.ACCX
 * multiplies 16 int8 pairs and adds their sum to the 40-bit ACCX
 * accumulator, so a dot product over IC channels is IC/16 instructions.
 *
 * Bit-exact with TFLM's reference ConvPerChannel:
 *   reference: acc = sum(w * (x + input_offset)) + bias
 *   here:      acc = sum(w * x) + (bias + input_offset * sum(w))
 * same integer; then the same MultiplyByQuantizedMultiplier, output
 * offset and clamp. Folded biases are computed once per layer.
 *
 * Alignment: EE.VLD.128 ignores the low 4 address bits. Activations come
 * from the TFLM arena (16-byte aligned) and IC % 16 == 0 keeps every row
 * aligned; weights live in memory-mapped flash inside the flatbuffer with
 * no alignment guarantee, so they're copied once into aligned PSRAM.
 */
#include <stdio.h>
#include <string.h>

#include "esp_heap_caps.h"
#include "esp_timer.h"

#include "tensorflow/lite/micro/kernels/conv.h"
#include "tensorflow/lite/micro/kernels/kernel_util.h"
#include "tensorflow/lite/micro/kernels/internal/common.h"

#include "pie_kernels.h"

namespace {

using namespace tflite;

TFLMRegistration ref;  /* TFLM's reference CONV_2D */
bool enabled = true;
bool verify = false;

/* Per-layer state, keyed by node (nodes are stable for the interpreter's
 * lifetime). */
struct Layer {
	const TfLiteNode *node;
	int ic, oc;
	int8_t *w;        /* [oc][ic], 16-byte aligned */
	int32_t *bias;    /* bias + input_offset * sum(w), per oc */
	/* last inference */
	bool pie;         /* took the PIE path */
	int64_t us;
	int mismatches;   /* verify mode */
};
constexpr int kMaxLayers = 64;
Layer layers[kMaxLayers];
int n_layers;

Layer *find_layer(const TfLiteNode *node)
{
	for (int i = 0; i < n_layers; i++)
		if (layers[i].node == node) return &layers[i];
	if (n_layers == kMaxLayers) return nullptr;
	layers[n_layers] = Layer{};
	layers[n_layers].node = node;
	return &layers[n_layers++];
}

/* 16*n int8 dot product on PIE. x and w 16-byte aligned. Sums here are
 * < 2^31 (256 * 128 * 128 = 2^22), so ACCX's low 32 bits (RUR.ACCX_0) are
 * the exact int32 result. */
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

bool pointwise_eligible(const TfLiteConvParams &p, const TfLiteEvalTensor *in,
			const TfLiteEvalTensor *filter, const TfLiteEvalTensor *out)
{
	if (in->type != kTfLiteInt8 || filter->type != kTfLiteInt8 || out->type != kTfLiteInt8) return false;
	if (p.stride_width != 1 || p.stride_height != 1) return false;
	if (p.dilation_width_factor != 1 || p.dilation_height_factor != 1) return false;
	const TfLiteIntArray *f = filter->dims;  /* OHWI */
	if (f->size != 4 || f->data[1] != 1 || f->data[2] != 1) return false;
	if (in->dims->data[0] != 1) return false;
	int ic = f->data[3];
	if (ic % 16 != 0 || in->dims->data[3] != ic) return false;
	if (((uintptr_t)in->data.int8 & 15) != 0) return false;
	return true;
}

bool prepare_layer(Layer *L, const TfLiteEvalTensor *filter, const TfLiteEvalTensor *bias,
		   const micro::OpDataConv &data)
{
	L->oc = filter->dims->data[0];
	L->ic = filter->dims->data[3];
	size_t n = (size_t)L->oc * L->ic;
	L->w = (int8_t *)heap_caps_aligned_alloc(16, n, MALLOC_CAP_SPIRAM);
	L->bias = (int32_t *)heap_caps_malloc(L->oc * sizeof(int32_t), MALLOC_CAP_SPIRAM);
	if (!L->w || !L->bias) return false;
	memcpy(L->w, filter->data.int8, n);
	const int32_t input_offset = -data.input_zero_point;
	for (int o = 0; o < L->oc; o++) {
		int32_t s = 0;
		for (int i = 0; i < L->ic; i++) s += L->w[o * L->ic + i];
		L->bias[o] = (bias ? bias->data.i32[o] : 0) + input_offset * s;
	}
	return true;
}

void pointwise_pie(const Layer *L, const micro::OpDataConv &data, const int8_t *in, int8_t *out, int pixels)
{
	const int ic = L->ic, oc = L->oc, n16 = ic / 16;
	const int32_t out_off = data.output_zero_point;
	const int32_t lo = data.output_activation_min, hi = data.output_activation_max;
	const int32_t *mult = data.per_channel_output_multiplier;
	const int32_t *shift = data.per_channel_output_shift;
	for (int p = 0; p < pixels; p++) {
		const int8_t *x = in + (size_t)p * ic;
		const int8_t *w = L->w;
		for (int o = 0; o < oc; o++, w += ic) {
			int32_t acc = dot_s8(x, w, n16) + L->bias[o];
			acc = MultiplyByQuantizedMultiplier(acc, mult[o], shift[o]);
			acc += out_off;
			acc = acc < lo ? lo : acc > hi ? hi : acc;
			*out++ = (int8_t)acc;
		}
	}
}

TfLiteStatus Eval(TfLiteContext *ctx, TfLiteNode *node)
{
	const auto &params = *static_cast<const TfLiteConvParams *>(node->builtin_data);
	const auto &data = *static_cast<const micro::OpDataConv *>(node->user_data);
	const TfLiteEvalTensor *in = micro::GetEvalInput(ctx, node, micro::kConvInputTensor);
	const TfLiteEvalTensor *filter = micro::GetEvalInput(ctx, node, micro::kConvWeightsTensor);
	const TfLiteEvalTensor *bias = node->inputs->size == 3
		? micro::GetEvalInput(ctx, node, micro::kConvBiasTensor) : nullptr;
	TfLiteEvalTensor *out = micro::GetEvalOutput(ctx, node, micro::kConvOutputTensor);

	Layer *L = find_layer(node);
	int64_t t0 = esp_timer_get_time();
	bool pie = enabled && L && pointwise_eligible(params, in, filter, out);
	if (pie && !L->w && !prepare_layer(L, filter, bias, data)) pie = false;
	if (!pie) {
		TfLiteStatus s = ref.invoke(ctx, node);
		if (L) { L->pie = false; L->us = esp_timer_get_time() - t0; L->mismatches = 0; }
		return s;
	}

	int pixels = out->dims->data[1] * out->dims->data[2];
	pointwise_pie(L, data, in->data.int8, out->data.int8, pixels);
	L->pie = true;
	L->us = esp_timer_get_time() - t0;
	L->mismatches = 0;

	if (verify) {
		size_t n = (size_t)pixels * L->oc;
		int8_t *mine = (int8_t *)heap_caps_malloc(n, MALLOC_CAP_SPIRAM);
		if (mine) {
			memcpy(mine, out->data.int8, n);
			ref.invoke(ctx, node);  /* reference result into the output tensor */
			for (size_t i = 0; i < n; i++) L->mismatches += mine[i] != out->data.int8[i];
			heap_caps_free(mine);
		}
	}
	return kTfLiteOk;
}

}  // namespace

TFLMRegistration PieRegister_CONV_2D()
{
	ref = tflite::Register_CONV_2D();
	TFLMRegistration r = ref;
	r.invoke = Eval;
	return r;
}

extern "C" void pie_set_enabled(bool on) { enabled = on; }
extern "C" bool pie_enabled(void) { return enabled; }
extern "C" void pie_set_verify(bool on) { verify = on; }

extern "C" void pie_print_stats(void)
{
	int64_t pie_us = 0, ref_us = 0;
	int n_pie = 0, bad = 0;
	printf("%-3s %-5s %8s %10s %s\n", "#", "path", "ic->oc", "ms", verify ? "mismatches" : "");
	for (int i = 0; i < n_layers; i++) {
		const Layer &L = layers[i];
		(L.pie ? pie_us : ref_us) += L.us;
		n_pie += L.pie;
		bad += L.mismatches;
		char ch[16] = "-";
		if (L.w) snprintf(ch, sizeof(ch), "%d->%d", L.ic, L.oc);
		printf("%-3d %-5s %8s %10.2f", i, L.pie ? "PIE" : "ref", ch, L.us / 1e3);
		if (verify && L.pie) printf(" %d", L.mismatches);
		printf("\n");
	}
	printf("CONV_2D: %d/%d layers on PIE (%.1f ms), %d on reference (%.1f ms)%s\n",
	       n_pie, n_layers, pie_us / 1e3, n_layers - n_pie, ref_us / 1e3,
	       verify ? (bad ? " -- VERIFY MISMATCH" : " -- verify: all PIE layers bit-exact") : "");
}
