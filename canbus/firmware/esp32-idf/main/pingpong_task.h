#pragma once
#include <stdint.h>

/*
 * Two-node bring-up exercise: a request/response frame pair over the
 * real CAN bus. Which side this node plays -- ping or pong -- comes from
 * identity.h's runtime `mode`, not a compile-time flag, so the exact
 * same binary runs both roles; "config set-mode" (see identity.c) picks
 * which one at runtime, and takes effect on this task's very next loop
 * iteration (no reboot). Requires NODE_ENABLE_CAN.
 *
 * This module only touches CAN + its own status snapshot below -- it
 * does not call lcd_display() itself. display_task owns the display (all
 * of it, one rotating "tab" per module with something to show); a "ping"
 * tab there reads pingpong_task_status_read() and formats it, the same
 * way display_task already reads state_counter_read() for its own
 * "counter" tab rather than state.c writing to the LCD directly.
 */

typedef enum {
	PINGPONG_STATUS_NONE = 0,    /* no exchange yet this boot */
	PINGPONG_STATUS_OK,          /* ping: got a matching pong. pong: replied to a ping */
	PINGPONG_STATUS_TIMEOUT,     /* ping only: no matching pong within PING_TIMEOUT_MS */
} pingpong_status_t;

void pingpong_task_init(void);
void pingpong_task(void *arg);

/* Mutex-protected snapshot of the most recent exchange -- rtt_ms is only
 * meaningful when status is PINGPONG_STATUS_OK and this node is currently
 * in ping mode (0 otherwise, e.g. a pong reply has no round trip of its
 * own to report). */
void pingpong_task_status_read(pingpong_status_t *status, uint32_t *seq, uint32_t *rtt_ms);
