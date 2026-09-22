#include <inttypes.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "can.h"
#include "metrics.h"
#include "pong_role.h"

static const char *TAG = "pong";

/* Scratch IDs in the unallocated gap (0x100-0x6FF) -- see
 * ../../../docs/can-message-spec.md. Not a real registered message.
 * Must match ping_role.c's copies of the same two defines. */
#define CAN_ID_PING 0x120
#define CAN_ID_PONG 0x121

static void status_set(const char *status, uint32_t seq, uint32_t rtt_ms)
{
	metrics_set_string("pong_role.status", status);
	metrics_set_int("pong_role.seq", (int32_t)seq);
	metrics_set_int("pong_role.rtt_ms", (int32_t)rtt_ms);
}

static uint32_t decode_seq(const twai_message_t *msg)
{
	return (uint32_t)msg->data[0] | ((uint32_t)msg->data[1] << 8) |
	       ((uint32_t)msg->data[2] << 16) | ((uint32_t)msg->data[3] << 24);
}

/* Waits indefinitely for a PING to echo back as PONG -- no bounded
 * timeout needed here; this task's whole identity is "be the pong
 * side", so there's nothing else to re-check per iteration. */
static void do_pong_wait(void)
{
	twai_message_t msg;
	if (!can_receive(&msg, portMAX_DELAY)) {
		return;
	}
	if (msg.identifier != CAN_ID_PING || msg.data_length_code < 4) {
		return;
	}

	uint32_t seq = decode_seq(&msg);
	can_send_u32(CAN_ID_PONG, seq);
	ESP_LOGI(TAG, "seq=%" PRIu32 " replied", seq);
	/* Not state_counter_increment() here -- can_rx_task already bumped
	 * it when this PING frame was received. */
	status_set("ok", seq, 0);
}

static void pong_task(void *arg)
{
	while (1) {
		do_pong_wait();
	}
}

void launch_pong_role(void)
{
	metrics_register_string("pong_role.status", "none");
	metrics_register_int("pong_role.seq", 0);
	metrics_register_int("pong_role.rtt_ms", 0);

	xTaskCreate(pong_task, "pong", 3072, NULL, 5, NULL);
}
