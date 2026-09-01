#include <inttypes.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "state.h"
#include "display_task.h"
#if NODE_ENABLE_CAN
#include "can.h"
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

void display_task_init(void)
{
	/* Nothing to set up -- just logs (and, if CAN is enabled, sends) on
	 * a timer. */
}

/* Logs the excitement counter's value every DISPLAY_MS (suppressed along
 * with everything else while CLI mode has the log level muted), and --
 * if this node has CAN -- broadcasts it too. can_send_u32() logs its own
 * failure reason, so nothing else to do here if it fails. */
void display_task(void *arg)
{
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(DISPLAY_MS));

		uint32_t counter = state_counter_read();
		ESP_LOGI(TAG, "excitement counter = %" PRIu32, counter);

#if NODE_ENABLE_CAN
		can_send_u32(DISPLAY_CAN_ID, counter);
#endif
	}
}
