#include <inttypes.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_timer.h"

#include "node_config.h"
#include "can.h"
#include "metrics.h"
#include "ping_role.h"

static const char *TAG = "ping";

/* Scratch IDs in the unallocated gap (0x100-0x6FF) -- see
 * ../../../docs/can-message-spec.md. Not a real registered message.
 * Must match pong_role.c's copies of the same two defines. */
#define CAN_ID_PING 0x120
#define CAN_ID_PONG 0x121

static void status_set(const char *status, uint32_t seq, uint32_t rtt_ms)
{
	metrics_set_string("ping_role.status", status);
	metrics_set_int("ping_role.seq", (int32_t)seq);
	metrics_set_int("ping_role.rtt_ms", (int32_t)rtt_ms);
}

static uint32_t decode_seq(const twai_message_t *msg)
{
	return (uint32_t)msg->data[0] | ((uint32_t)msg->data[1] << 8) |
	       ((uint32_t)msg->data[2] << 16) | ((uint32_t)msg->data[3] << 24);
}

/* Sends one PING, waits up to PING_TIMEOUT_MS for the matching PONG --
 * any other frame seen in that window (including a stale PONG replying
 * to an earlier, already-timed-out ping) is drained and ignored rather
 * than treated as a wrong answer. */
static void do_ping(uint32_t seq)
{
	int64_t sent_us = esp_timer_get_time();
	can_send_u32(CAN_ID_PING, seq);

	TickType_t deadline = xTaskGetTickCount() + pdMS_TO_TICKS(PING_TIMEOUT_MS);
	while (xTaskGetTickCount() < deadline) {
		twai_message_t msg;
		if (!can_receive(&msg, deadline - xTaskGetTickCount())) {
			break;
		}
		if (msg.identifier != CAN_ID_PONG || msg.data_length_code < 4 ||
		    decode_seq(&msg) != seq) {
			continue;
		}

		uint32_t rtt_ms = (uint32_t)((esp_timer_get_time() - sent_us) / 1000);
		ESP_LOGI(TAG, "seq=%" PRIu32 " rtt=%" PRIu32 "ms", seq, rtt_ms);
		/* Not state_counter_increment() here -- can_rx_task already
		 * bumped it the moment this PONG frame was received; matching
		 * it to this ping is this task's own bookkeeping, not a
		 * second countable event. */
		status_set("ok", seq, rtt_ms);
		return;
	}

	ESP_LOGW(TAG, "seq=%" PRIu32 " timed out", seq);
	status_set("timeout", seq, 0);
}

static void ping_task(void *arg)
{
	uint32_t seq = 0;

	while (1) {
		do_ping(seq++);
		vTaskDelay(pdMS_TO_TICKS(PING_PERIOD_MS));
	}
}

void launch_ping_role(void)
{
	metrics_register_string("ping_role.status", "none");
	metrics_register_int("ping_role.seq", 0);
	metrics_register_int("ping_role.rtt_ms", 0);

	xTaskCreate(ping_task, "ping", 3072, NULL, 5, NULL);
}
