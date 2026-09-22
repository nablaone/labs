#include <inttypes.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "can.h"
#include "metrics.h"
#include "pot_collector_role.h"

static const char *TAG = "pot_collector";

/* Scratch ID in the unallocated gap (0x100-0x6FF) -- see
 * ../../../docs/can-message-spec.md. Not a real registered message.
 * Must match pot_sender_role.c's copy of the same define. */
#define CAN_ID_POT_VALUE 0x130

static uint32_t decode_u32(const twai_message_t *msg)
{
	return (uint32_t)msg->data[0] | ((uint32_t)msg->data[1] << 8) |
	       ((uint32_t)msg->data[2] << 16) | ((uint32_t)msg->data[3] << 24);
}

static void pot_collector_task(void *arg)
{
	while (1) {
		twai_message_t msg;
		if (!can_receive(&msg, portMAX_DELAY)) {
			continue;
		}
		if (msg.identifier != CAN_ID_POT_VALUE || msg.data_length_code < 4) {
			continue;
		}

		uint32_t raw = decode_u32(&msg);
		ESP_LOGI(TAG, "raw=%" PRIu32, raw);
		/* Not state_counter_increment() here -- can_rx_task already
		 * bumped it when this frame was received. */
		metrics_set_int("pot_collector.raw", (int32_t)raw);
	}
}

void launch_pot_collector_role(void)
{
	metrics_register_int("pot_collector.raw", 0);

	xTaskCreate(pot_collector_task, "pot_collector", 3072, NULL, 5, NULL);
}
