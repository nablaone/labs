#include <stdio.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_console.h"
#include "argtable3/argtable3.h"

#include "node_config.h"
#include "state.h"
#include "heartbeat_task.h"

void heartbeat_task_init(void)
{
	/* No hardware to set up -- heartbeat is purely a software timer. */
}

/* Increments the counter on its own, at a period settable at runtime via
 * the "rate" CLI command -- re-read each cycle rather than latched, so a
 * rate change takes effect on the next tick without needing to interrupt
 * an in-progress wait. */
void heartbeat_task(void *arg)
{
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(state_heartbeat_period_read()));
		state_counter_increment();
	}
}

static struct {
	struct arg_int *n;
	struct arg_end *end;
} rate_args;

static int cmd_rate(int argc, char **argv)
{
	int nerrors = arg_parse(argc, argv, (void **)&rate_args);
	if (nerrors) {
		arg_print_errors(stderr, rate_args.end, argv[0]);
		return 1;
	}

	int n = rate_args.n->ival[0];
	if (n <= 0) {
		printf("N must be a positive integer (period = N * 100ms)\n");
		return 1;
	}

	state_heartbeat_period_set((uint32_t)n * 100);
	printf("heartbeat period set to %dms\n", n * 100);
	return 0;
}

void heartbeat_task_register_cli_commands(void)
{
	rate_args.n = arg_int1(NULL, NULL, "<N>", "heartbeat period, in units of 100ms");
	rate_args.end = arg_end(1);
	const esp_console_cmd_t rate_cmd = {
		.command = "rate",
		.help = "Set the heartbeat period to N*100ms",
		.func = &cmd_rate,
		.argtable = &rate_args,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&rate_cmd));
}
