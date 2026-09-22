#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "state.h"
#include "console.h"
#include "identity.h"
#include "heartbeat_task.h"
#include "display_task.h"
#include "can.h"
#include "lcd_task.h"
#include "pingpong_task.h"

static const char *TAG = "main";

/*
 * Orchestration only -- each module owns its own init/task-loop/CLI-
 * registration. Common hardware (heartbeat+LED, display, LCD, CAN) is
 * always brought up; which role this node plays is runtime/NVRAM-backed
 * (identity.c's role) rather than a compile-time choice, so it's decided
 * last, once, via the switch below -- "config set-role" reboots the
 * board immediately (see identity.c) precisely because this decision is
 * only ever made here, at boot.
 */
void app_main(void)
{
	state_init();
	identity_init();
	console_init();

	state_register_cli_commands();
	identity_register_cli_commands();

	heartbeat_task_init();
	display_task_init();
	lcd_task_init();

	ESP_LOGI(TAG, "CAN self-test: %s", can_run_selftest() ? "PASS" : "FAIL");
	can_register_cli_commands();

	lcd_task_register_cli_commands();

	xTaskCreate(heartbeat_task, "heartbeat", 3072, NULL, 5, NULL);
	xTaskCreate(display_task, "display", 3072, NULL, 5, NULL);
	xTaskCreate(lcd_task, "lcd", 3072, NULL, 5, NULL);
	xTaskCreate(can_rx_task, "can_rx", 3072, NULL, 5, NULL);

	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);

	/* Which task starts is decided once here, at boot, from NVRAM-backed
	 * identity_role_read() -- ping_task()/pong_task() don't re-check role
	 * themselves; identity_role_set() reboots the board on every role
	 * change (see identity.c) precisely so this switch always reflects
	 * the current role. */
	identity_role_t role;
	if (identity_role_read(&role)) {
		switch (role) {
		case IDENTITY_ROLE_PING:
			ESP_LOGI(TAG, "role: ping");
			ping_task_init();
			xTaskCreate(ping_task, "ping", 3072, NULL, 5, NULL);
			break;
		case IDENTITY_ROLE_PONG:
			ESP_LOGI(TAG, "role: pong");
			pong_task_init();
			xTaskCreate(pong_task, "pong", 3072, NULL, 5, NULL);
			break;
		default:
			ESP_LOGE(TAG, "role: unknown value %d -- no task started", (int)role);
			break;
		}
	} else {
		ESP_LOGE(TAG, "role: not configured -- run 'config set-role ping|pong'");
	}
}
