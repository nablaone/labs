#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "state.h"
#include "console.h"
#include "identity.h"
#include "metrics.h"
#include "heartbeat_task.h"
#include "display_task.h"
#include "can.h"
#include "ping_role.h"
#include "pong_role.h"

static const char *TAG = "main";

/*
 * Orchestration only -- each module owns its own init/task-loop/CLI-
 * registration. Common hardware (heartbeat+LED, display/LCD, CAN) is
 * always brought up; which role this node plays is runtime/NVRAM-backed
 * (identity.c's role) rather than a compile-time choice, so it's decided
 * last, once, by the switch below -- the single place that dispatches to
 * launch_ping_role()/launch_pong_role(), each of which unconditionally
 * registers its metrics and starts its task once called. "config
 * set-role" reboots the board immediately (see identity.c) precisely
 * because this dispatch only ever happens here, at boot.
 */
void app_main(void)
{
	metrics_init();
	metrics_register_string("version", FIRMWARE_VERSION);
	state_init();
	identity_init();
	console_init();

	state_register_cli_commands();
	identity_register_cli_commands();
	metrics_register_cli_commands();

	heartbeat_task_init();
	display_task_init();

	ESP_LOGI(TAG, "CAN self-test: %s", can_run_selftest() ? "PASS" : "FAIL");
	can_register_cli_commands();

	xTaskCreate(heartbeat_task, "heartbeat", 3072, NULL, 5, NULL);
	xTaskCreate(display_task, "display", 3072, NULL, 5, NULL);
	xTaskCreate(can_rx_task, "can_rx", 3072, NULL, 5, NULL);

	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);

	/* Single dispatch point: which role's launcher (if any) gets called
	 * is decided once here, at boot, from NVRAM-backed identity_role_read()
	 * -- identity_role_set() reboots the board on every role change, so
	 * this always reflects the current role. */
	identity_role_t role;
	if (identity_role_read(&role)) {
		switch (role) {
		case IDENTITY_ROLE_PING:
			ESP_LOGI(TAG, "role: ping");
			launch_ping_role();
			break;
		case IDENTITY_ROLE_PONG:
			ESP_LOGI(TAG, "role: pong");
			launch_pong_role();
			break;
		default:
			ESP_LOGE(TAG, "role: unknown value %d -- no task started", (int)role);
			break;
		}
	} else {
		ESP_LOGE(TAG, "role: not configured -- run 'config set-role ping|pong'");
	}
}
