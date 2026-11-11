#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_timer.h"
#include <stdio.h>

#include "pie_parallel.h"

namespace {

const char *TAG = "pie_par";

struct Worker {
	TaskHandle_t task;
	SemaphoreHandle_t done;
	pie_range_fn fn;
	void *ctx;
	int begin, end;
	int64_t us;  /* time the last job took on this worker */
};
Worker workers[2];
int cores = 2;
/* split balance, accumulated until pie_parallel_reset_stats() */
int64_t self_us, worker_us, wait_us;
int jobs;

void worker_task(void *arg)
{
	Worker *w = static_cast<Worker *>(arg);
	for (;;) {
		ulTaskNotifyTake(pdTRUE, portMAX_DELAY);
		int64_t t0 = esp_timer_get_time();
		w->fn(w->ctx, w->begin, w->end);
		w->us = esp_timer_get_time() - t0;
		xSemaphoreGive(w->done);
	}
}

}  // namespace

void pie_parallel_init(void)
{
	static const char *const names[2] = { "pie_w0", "pie_w1" };
	for (int c = 0; c < 2; c++) {
		workers[c].done = xSemaphoreCreateBinary();
		if (!workers[c].done ||
		    xTaskCreatePinnedToCore(worker_task, names[c], 6144, &workers[c], 3, &workers[c].task, c) != pdPASS) {
			ESP_LOGW(TAG, "worker on core %d failed, running single-core", c);
			workers[c].task = nullptr;
			cores = 1;
		}
	}
}

void pie_parallel_set_cores(int n) { cores = n >= 2 ? 2 : 1; }
int pie_parallel_cores(void) { return cores; }

void pie_parallel_run(int n, pie_range_fn fn, void *ctx)
{
	Worker &w = workers[!xPortGetCoreID()];
	if (cores < 2 || n < 2 || !w.task) {
		fn(ctx, 0, n);
		return;
	}
	const int split = n / 2;
	w.fn = fn;
	w.ctx = ctx;
	w.begin = split;
	w.end = n;
	xTaskNotifyGive(w.task);  /* includes the memory barrier for the fields above */
	int64_t t0 = esp_timer_get_time();
	fn(ctx, 0, split);
	int64_t t1 = esp_timer_get_time();
	xSemaphoreTake(w.done, portMAX_DELAY);
	self_us += t1 - t0;
	worker_us += w.us;
	wait_us += esp_timer_get_time() - t1;
	jobs++;
}

void pie_parallel_reset_stats(void) { self_us = worker_us = wait_us = 0; jobs = 0; }

void pie_parallel_print_stats(void)
{
	if (!jobs) return;
	printf("2-core split: %d jobs, caller's half %.1f ms, worker's half %.1f ms, caller waited %.1f ms\n",
	       jobs, self_us / 1e3, worker_us / 1e3, wait_us / 1e3);
}
