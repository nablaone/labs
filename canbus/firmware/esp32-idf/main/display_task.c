#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "state.h"
#include "display_task.h"
#if NODE_ENABLE_CAN
#include "can.h"
#endif
#if NODE_ENABLE_LCD
#include "lcd_task.h"
#endif

static const char *TAG = "display";

/* Deliberately high in the 11-bit standard ID space -- CAN arbitration is
 * lowest-ID-wins, so a high numeric ID is *lowest* priority, appropriate
 * for a non-critical periodic debug broadcast that should never delay
 * real control traffic. Sits in the diagnostics band
 * (../../../docs/can-message-spec.md's 0x700-0x7FF) but away from that
 * doc's 0x7NN NODE_HEARTBEAT pattern -- this is an experimental counter
 * broadcast, not that registered message. */
#define DISPLAY_CAN_ID 0x7F0

/* Parameters this task cycles through, one per DISPLAY_CYCLE_MS. */
typedef enum {
	DISPLAY_PARAM_VERSION = 0,
	DISPLAY_PARAM_COUNTER,
	DISPLAY_PARAM_HELLO,
	DISPLAY_PARAM_COUNT,
} display_param_t;

void display_task_init(void)
{
	/* Nothing to set up -- just cycles through parameters on a timer. */
}

/* Cycles through version / counter / hello every DISPLAY_CYCLE_MS. Each
 * is logged (suppressed along with everything else while CLI mode has
 * the log level muted) and, if this node has an LCD, shown there too as
 * "<name>" on line 1 / "<value>" on line 2 via lcd_display() -- the same
 * public API any other module would use, this task has no special
 * access to the display. Only the counter's turn also broadcasts over
 * CAN -- can_send_u32() logs its own failure reason, so nothing else to
 * do here if it fails. */
void display_task(void *arg)
{
	display_param_t param = DISPLAY_PARAM_VERSION;

	while (1) {
		switch (param) {
		case DISPLAY_PARAM_VERSION:
			ESP_LOGI(TAG, "version = %s", FIRMWARE_VERSION);
#if NODE_ENABLE_LCD
			lcd_display("version", FIRMWARE_VERSION);
#endif
			break;

		case DISPLAY_PARAM_COUNTER: {
			uint32_t counter = state_counter_read();
			ESP_LOGI(TAG, "counter = %" PRIu32, counter);
#if NODE_ENABLE_LCD
			char value[12];
			snprintf(value, sizeof(value), "%" PRIu32, counter);
			lcd_display("counter", value);
#endif
#if NODE_ENABLE_CAN
			can_send_u32(DISPLAY_CAN_ID, counter);
#endif
			break;
		}

		case DISPLAY_PARAM_HELLO:
			ESP_LOGI(TAG, "hello = world");
#if NODE_ENABLE_LCD
			lcd_display("hello", "world");
#endif
			break;

		default:
			break;
		}

		param = (param + 1) % DISPLAY_PARAM_COUNT;
		vTaskDelay(pdMS_TO_TICKS(DISPLAY_CYCLE_MS));
	}
}
