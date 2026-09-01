#include <stdio.h>
#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include "esp_console.h"

#include "node_config.h"
#include "state.h"

static struct {
	uint32_t counter;
	uint32_t heartbeat_period_ms;
} state = {
	.counter = 0,
	.heartbeat_period_ms = HEARTBEAT_MS,
};
static SemaphoreHandle_t state_mutex;

void state_init(void)
{
	state_mutex = xSemaphoreCreateMutex();
}

uint32_t state_counter_read(void)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	uint32_t value = state.counter;
	xSemaphoreGive(state_mutex);
	return value;
}

void state_counter_increment(void)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	state.counter++;
	xSemaphoreGive(state_mutex);
}

uint32_t state_heartbeat_period_read(void)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	uint32_t value = state.heartbeat_period_ms;
	xSemaphoreGive(state_mutex);
	return value;
}

void state_heartbeat_period_set(uint32_t ms)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	state.heartbeat_period_ms = ms;
	xSemaphoreGive(state_mutex);
}

static int cmd_counter(int argc, char **argv)
{
	printf("%" PRIu32 "\n", state_counter_read());
	return 0;
}

void state_register_cli_commands(void)
{
	const esp_console_cmd_t counter_cmd = {
		.command = "counter",
		.help = "Show the current excitement counter value",
		.func = &cmd_counter,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&counter_cmd));
}
