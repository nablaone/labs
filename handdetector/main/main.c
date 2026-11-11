#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_chip_info.h"
#include "esp_psram.h"
#include "esp_flash.h"

#include "node_config.h"
#include "console.h"
#include "heartbeat_task.h"
#include "pmic.h"
#include "camera.h"
#include "ble_link.h"

static const char *TAG = "main";

static void log_board(void)
{
	esp_chip_info_t chip;
	esp_chip_info(&chip);
	uint32_t flash = 0;
	esp_flash_get_size(NULL, &flash);
	ESP_LOGI(TAG, "ESP32-S3 rev v%d.%d, %d cores, flash %lu MB, PSRAM %u MB",
		 chip.revision / 100, chip.revision % 100, chip.cores,
		 (unsigned long)(flash >> 20), (unsigned)(esp_psram_get_size() >> 20));
}

/*
 * Orchestration only -- each module owns its own init/task/CLI. Camera
 * detection failing isn't fatal: the console still comes up so it can be
 * debugged with `i2cscan` / `cam init`.
 */
void app_main(void)
{
	console_init();
	log_board();

	heartbeat_task_init();
	pmic_register_cli_commands();
	camera_register_cli_commands();
	camera_init();
	ble_link_init();
	ble_link_register_cli_commands();

	xTaskCreate(heartbeat_task, "heartbeat", 2048, NULL, 5, NULL);
	xTaskCreate(ble_link_task, "ble_link", 4096, NULL, 5, NULL);
	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);

	ESP_LOGI(TAG, "handdetector %s up -- press Enter for the CLI", FIRMWARE_VERSION);
}
