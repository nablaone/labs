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

#include "tensorflow/lite/micro/micro_common.h"

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
}
