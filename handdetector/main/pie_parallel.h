#pragma once

/*
 * Two-core split for the PIE/LUT kernels.
 *
 * One worker task per core waits for jobs. pie_parallel_for(n, f) splits
 * [0, n) in two: the calling task runs f(0, n/2) on its own core while the
 * worker on the *other* core runs f(n/2, n). So it parallelizes whichever
 * core the caller is on: normally the detector task on core 1 (worker on
 * core 0, otherwise idle apart from BLE and the console), and `det golden`
 * from the console on core 0 (worker on core 1).
 *
 * f must only write its own part of the output; anything shared (tables,
 * padded inputs, folded biases) is prepared before the call. PIE
 * accumulators (ACCX, QACC) are per core, so each half has its own.
 *
 * Workers run at priority 3: above the detector task (2), below the BLE,
 * console and heartbeat tasks (5) and the NimBLE host, so the link stays
 * responsive and an inference just takes a little longer when it's busy.
 */

void pie_parallel_init(void);
void pie_parallel_set_cores(int n);  /* 1 = everything on the caller's core */
int pie_parallel_cores(void);
void pie_parallel_reset_stats(void);
void pie_parallel_print_stats(void);

typedef void (*pie_range_fn)(void *ctx, int begin, int end);
void pie_parallel_run(int n, pie_range_fn fn, void *ctx);

template <typename F>
inline void pie_parallel_for(int n, const F &f)
{
	pie_parallel_run(n, [](void *c, int b, int e) { (*static_cast<const F *>(c))(b, e); },
			 const_cast<F *>(&f));
}
