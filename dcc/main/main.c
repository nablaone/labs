#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "console.h"
#include "heartbeat_task.h"
#include "dcc.h"
#include "ble_link.h"

static const char *TAG = "main";

/*
 * Orchestration only -- each module owns its own init/task-loop/CLI-
 * registration. The remaining decoder modules (accessory address
 * matching, outputs -- see ../docs/dcc-accessory-primer.md) slot in here
 * the same way: an _init() and _register_cli_commands() call, plus an
 * xTaskCreate() if they need one.
 */
void app_main(void)
{
	console_init();

	heartbeat_task_init();
	dcc_init();
	dcc_register_cli_commands();
	ble_link_init();
	ble_link_register_cli_commands();

	xTaskCreate(heartbeat_task, "heartbeat", 2048, NULL, 5, NULL);
	xTaskCreate(ble_link_tx_task, "ble_tx", 4096, NULL, 5, NULL);
	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);

	ESP_LOGI(TAG, "dcc %s up -- press Enter for the CLI", FIRMWARE_VERSION);
}
