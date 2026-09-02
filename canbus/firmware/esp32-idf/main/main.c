#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "state.h"
#include "console.h"
#include "identity.h"
#if NODE_ENABLE_LED
#include "led_task.h"
#endif
#if NODE_ENABLE_HEARTBEAT
#include "heartbeat_task.h"
#endif
#if NODE_ENABLE_BUTTON
#include "button_task.h"
#endif
#if NODE_ENABLE_DISPLAY
#include "display_task.h"
#endif
#if NODE_ENABLE_CAN
#include "can.h"
#endif
#if NODE_ENABLE_LCD
#include "lcd_task.h"
#endif
#if NODE_ENABLE_PINGPONG
#include "pingpong_task.h"
#endif

static const char *TAG = "main";

/*
 * Orchestration only -- each module owns its own init/task-loop/CLI-
 * registration (see node_config.h for the enable flags and pin
 * assignments a new node edits). Keeps this file identical across nodes;
 * only node_config.h and which modules exist need to change.
 */
void app_main(void)
{
	state_init();
	identity_init();
	console_init();

#if NODE_ENABLE_LED
	led_task_init();
#endif
#if NODE_ENABLE_HEARTBEAT
	heartbeat_task_init();
#endif
#if NODE_ENABLE_BUTTON
	button_task_init();
#endif
#if NODE_ENABLE_DISPLAY
	display_task_init();
#endif
#if NODE_ENABLE_LCD
	lcd_task_init();
#endif
#if NODE_ENABLE_PINGPONG
	pingpong_task_init();
#endif

#if NODE_ENABLE_CAN
	ESP_LOGI(TAG, "CAN self-test: %s", can_run_selftest() ? "PASS" : "FAIL");
	can_register_cli_commands();
#endif

	state_register_cli_commands();
	identity_register_cli_commands();
#if NODE_ENABLE_HEARTBEAT
	heartbeat_task_register_cli_commands();
#endif
#if NODE_ENABLE_LCD
	lcd_task_register_cli_commands();
#endif

#if NODE_ENABLE_LED
	xTaskCreate(led_task, "led", 3072, NULL, 5, NULL);
#endif
#if NODE_ENABLE_HEARTBEAT
	xTaskCreate(heartbeat_task, "heartbeat", 3072, NULL, 5, NULL);
#endif
#if NODE_ENABLE_BUTTON
	xTaskCreate(button_task, "button", 3072, NULL, 5, NULL);
#endif
#if NODE_ENABLE_DISPLAY
	xTaskCreate(display_task, "display", 3072, NULL, 5, NULL);
#endif
#if NODE_ENABLE_LCD
	xTaskCreate(lcd_task, "lcd", 3072, NULL, 5, NULL);
#endif
#if NODE_ENABLE_CAN
	xTaskCreate(can_rx_task, "can_rx", 3072, NULL, 5, NULL);
#endif
#if NODE_ENABLE_PINGPONG
	xTaskCreate(pingpong_task, "pingpong", 3072, NULL, 5, NULL);
#endif

	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);
}
