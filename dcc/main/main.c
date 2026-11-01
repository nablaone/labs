#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "console.h"
#include "heartbeat_task.h"

static const char *TAG = "main";

/*
 * Orchestration only -- each module owns its own init/task-loop/CLI-
 * registration. The DCC decoder modules (signal capture, packet decoder,
 * accessory outputs -- see ../docs/README.md) slot in here the same way:
 * an _init() and _register_cli_commands() call, plus an xTaskCreate().
 */
void app_main(void)
{
	console_init();

	heartbeat_task_init();

	xTaskCreate(heartbeat_task, "heartbeat", 2048, NULL, 5, NULL);
	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);

	ESP_LOGI(TAG, "dcc %s up -- press Enter for the CLI", FIRMWARE_VERSION);
}
