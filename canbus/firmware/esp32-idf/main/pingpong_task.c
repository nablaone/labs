#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/semphr.h"
#include "esp_log.h"
#include "esp_timer.h"

#include "node_config.h"
#include "can.h"
#include "pingpong_task.h"

static const char *TAG = "pingpong";

/* Mutex-protected, same pattern as state.c -- display_task's "ping" tab
 * reads this via pingpong_task_status_read() rather than this module
 * writing to the LCD itself. */
static struct {
	pingpong_status_t status;
	uint32_t seq;
	uint32_t rtt_ms;
} pingpong_state;
static SemaphoreHandle_t pingpong_mutex;

static void status_set(pingpong_status_t status, uint32_t seq, uint32_t rtt_ms)
{
	xSemaphoreTake(pingpong_mutex, portMAX_DELAY);
	pingpong_state.status = status;
	pingpong_state.seq = seq;
	pingpong_state.rtt_ms = rtt_ms;
	xSemaphoreGive(pingpong_mutex);
}

void pingpong_task_status_read(pingpong_status_t *status, uint32_t *seq, uint32_t *rtt_ms)
{
	xSemaphoreTake(pingpong_mutex, portMAX_DELAY);
	*status = pingpong_state.status;
	*seq = pingpong_state.seq;
	*rtt_ms = pingpong_state.rtt_ms;
	xSemaphoreGive(pingpong_mutex);
}

/* Scratch IDs in the unallocated gap (0x100-0x6FF) --
 * see ../../../docs/can-message-spec.md. Not a real registered message. */
#define CAN_ID_PING 0x120
#define CAN_ID_PONG 0x121

void ping_task_init(void)
{
	pingpong_mutex = xSemaphoreCreateMutex();
}

void pong_task_init(void)
{
	pingpong_mutex = xSemaphoreCreateMutex();
}

static uint32_t decode_seq(const twai_message_t *msg)
{
	return (uint32_t)msg->data[0] | ((uint32_t)msg->data[1] << 8) |
	       ((uint32_t)msg->data[2] << 16) | ((uint32_t)msg->data[3] << 24);
}

/* Sends one PING, waits up to PING_TIMEOUT_MS for the matching PONG --
 * any other frame seen in that window (including a stale PONG replying
 * to an earlier, already-timed-out ping) is drained and ignored rather
 * than treated as a wrong answer -- and logs/publishes the round trip
 * for display_task's "ping" tab (see pingpong_task_status_read()). */
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
		status_set(PINGPONG_STATUS_OK, seq, rtt_ms);
		return;
	}

	ESP_LOGW(TAG, "seq=%" PRIu32 " timed out", seq);
	status_set(PINGPONG_STATUS_TIMEOUT, seq, 0);
}

/* Waits indefinitely for a PING to echo back as PONG -- no bounded
 * timeout needed here (unlike the old combined task) since this task's
 * whole identity is "be the pong side"; there's no role to re-check
 * per iteration any more. */
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
	status_set(PINGPONG_STATUS_OK, seq, 0);
}

void ping_task(void *arg)
{
	uint32_t seq = 0;

	while (1) {
		do_ping(seq++);
		vTaskDelay(pdMS_TO_TICKS(PING_PERIOD_MS));
	}
}

void pong_task(void *arg)
{
	while (1) {
		do_pong_wait();
	}
}
