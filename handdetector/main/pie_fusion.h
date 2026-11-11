#pragma once

/*
 * Operator fusion: CONV_2D -> ADD -> PRELU in one pass.
 *
 * The palm model's blocks end in a pointwise conv whose output goes only
 * into a residual ADD, whose output goes only into a PRELU (23 times; 3
 * more convs feed a PRELU directly). Run separately, each op is a full
 * pass over PSRAM: the conv writes its output, ADD reads it and writes
 * another tensor, PRELU reads that and writes a third. Fused, the conv's
 * output step computes, per element,
 *
 *   c   = requant(conv accumulator)                 -- what CONV would store
 *   a   = requant(r_skip[skip] + r_conv[c])         -- what ADD would store
 *   out = prelu(a, channel)                         -- what PRELU would store
 *
 * and writes only `out`, so the result is bit-exact with the unfused ops.
 *
 * How the chain is learned: the first inference runs unfused. Each conv
 * registers its output tensor; ADD and PRELU, when their input is such a
 * tensor with exactly one consumer in the graph, record their tables and
 * parameters in the conv's entry. From then on the conv does the whole
 * chain (marking the entry `pending`), and ADD/PRELU see `pending` and
 * return immediately.
 *
 * Safety: TFLM's planner may place the final tensor on top of memory that
 * is still live while the conv runs (its input), because it assumes the
 * conv is done before PRELU starts. The conv checks the real buffer
 * addresses and only fuses if the final tensor doesn't overlap its input
 * and either doesn't overlap the skip tensor or sits exactly on it (an
 * in-place, same-index update). Otherwise that block runs unfused.
 *
 * Off in verify mode (`det pie verify on` compares each op's own output,
 * which fusion never writes) and with `det pie fuse off`.
 */

#include <stdint.h>

struct PieFused {
	int conv_out;            /* key: tensor index the conv produces */
	/* ADD step (residual) */
	bool add;
	int add_out, skip;       /* tensor indices */
	int32_t r_skip[256];     /* per-input rescale tables, index v + 128 */
	int32_t r_conv[256];
	int32_t om, os, oo, lo, hi;
	/* PRELU step, TFLM's arithmetic (no table: a [256][C] table is up to
	 * 64 KB of scattered reads, which evicts the conv's weights from the
	 * 32 KB data cache):
	 *   v = p_in_off + x
	 *   y = v >= 0 ? MBQM(v, p_m1, p_s1) : MBQM(v * p_alpha[c], p_m2, p_s2)
	 *   y = clamp(y + p_out_off, -128, 127) */
	bool prelu;
	int prelu_out;
	int C;
	int32_t p_in_off, p_out_off, p_m1, p_s1, p_m2, p_s2;
	int32_t *p_alpha;        /* alpha[c] + alpha_offset, C entries */
	/* this inference: the conv already produced the final tensor */
	bool pending;
};

/* Consumer counts per tensor, from the model flatbuffer (tflite::Model *). */
void pie_fusion_set_model(const void *model);
int pie_fusion_consumers(int tensor);

PieFused *pie_fusion_get(int conv_out, bool create);
PieFused *pie_fusion_by_add_out(int tensor);

void pie_fusion_set_enabled(bool on);
bool pie_fusion_enabled(void);
void pie_fusion_print(void);
