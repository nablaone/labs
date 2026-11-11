#pragma once

/*
 * Hand-written int8 kernels for the ESP32-S3's PIE SIMD unit (Processor
 * Instruction Extensions, ESP32-S3 TRM chapter 1), plugged into TFLM by
 * wrapping its reference op registrations: Prepare is TFLM's own (so the
 * quantization parameters are identical), Invoke takes the PIE path when a
 * layer fits and falls back to the reference kernel otherwise. The
 * results are bit-exact with the reference kernels (`det golden`).
 *
 * Not portable on purpose: inline assembly using the EE.* instructions.
 */

#include <stdint.h>

#include "tensorflow/lite/micro/micro_common.h"

/*
 * Exact, inlined replacement for TFLM's MultiplyByQuantizedMultiplier()
 * (the default, double-rounding variant), which is a non-inlined call
 * doing a 64-bit division: ~82 cycles per output on the S3, the largest
 * single cost in the network.
 *
 *   reference: RoundingDivideByPOT(SaturatingRoundingDoublingHighMul(
 *                  x << left_shift, M), right_shift)
 *
 * SaturatingRoundingDoublingHighMul(a, M) = trunc((a*M + nudge) / 2^31)
 * with nudge = 2^30 for a*M >= 0 and 1 - 2^30 otherwise. For a*M < 0 the
 * numerator is negative, and trunc(n / d) = floor((n + d - 1) / d) there,
 * which makes both cases floor((a*M + 2^30) / 2^31): one 32x32->64
 * multiply, an add and an arithmetic shift. (Its saturation case, a = M =
 * INT32_MIN, can't happen: M >= 0.)
 *
 * RoundingDivideByPOT(x, e) rounds half away from zero:
 * (x + 2^(e-1) - (x < 0)) >> e for e >= 1. x here is at most |acc|
 * (M < 2^31), far from overflowing the add.
 *
 * `det pie bench` checks it against TFLM's function on random inputs, and
 * `det pie verify on` checks every layer's output.
 */
static inline int32_t pie_mbqm(int32_t x, int32_t m, int shift)
{
	const int32_t a = shift > 0 ? (int32_t)((uint32_t)x << shift) : x;
	int32_t hi = (int32_t)(((int64_t)a * m + (1 << 30)) >> 31);
	if (shift < 0) {
		const int e = -shift;
		hi = (hi + (1 << (e - 1)) - (hi < 0)) >> e;
	}
	return hi;
}

TFLMRegistration PieRegister_CONV_2D();
TFLMRegistration PieRegister_DEPTHWISE_CONV_2D();
/* Elementwise ops via lookup tables (pie_elementwise.cc). */
TFLMRegistration PieRegister_ABS();
TFLMRegistration PieRegister_RELU();
TFLMRegistration PieRegister_ADD();
TFLMRegistration PieRegister_SUB();
TFLMRegistration PieRegister_MUL();
TFLMRegistration PieRegister_PRELU();
void pie_elementwise_reset_stats(void);
void pie_elementwise_print_stats(void);
/* MAX_POOL_2D, PAD, RESIZE_BILINEAR (pie_data.cc). */
TFLMRegistration PieRegister_MAX_POOL_2D();
TFLMRegistration PieRegister_PAD();
TFLMRegistration PieRegister_RESIZE_BILINEAR();
void pie_data_reset_stats(void);
void pie_data_print_stats(void);

extern "C" {
/* Runtime switch (`det pie on|off`): off = every layer uses the
 * reference kernel, for A/B timing. */
void pie_set_enabled(bool on);
bool pie_enabled(void);
/* Verify mode (`det pie verify`): after each PIE layer, rerun the
 * reference kernel on the same input and count differing output bytes. */
void pie_set_verify(bool on);
bool pie_verify_enabled(void);
/* Per-layer stats of the last inference. */
void pie_print_stats(void);
/* Cycle counts of the kernel building blocks (`det pie bench`). */
void pie_microbench(void);
}
