#pragma once
#include <stdint.h>

/*
 * Two-node bring-up exercise: a request/response frame pair over the
 * real CAN bus. Which side this node plays -- ping or pong -- comes from
 * identity.h's runtime `role`, not a compile-time flag, so the exact same
 * binary runs both roles; main.c reads that role once at boot and starts
 * ping_task() or pong_task() accordingly (its switch also logs an error
 * if the role is unset or unrecognized). Since the choice is made once at
 * boot, not re-checked per loop, "config set-role" (see identity.c)
 * reboots the board immediately rather than switching roles live.
 * Requires NODE_ENABLE_CAN.
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

/* Sends a PING every PING_PERIOD_MS and waits for the matching PONG.
 * ping_task_init() sets up the shared status mutex below and registers
 * the pingpong.status/pingpong.seq/pingpong.rtt_ms metrics (see
 * metrics.c) -- call whichever of this or pong_task_init() matches the
 * task main.c is about to start; either one is enough, both do the same
 * thing. */
void ping_task_init(void);
void ping_task(void *arg);

/* Replies to every PING it sees. pong_task_init() is ping_task_init()'s
 * counterpart -- see above. */
void pong_task_init(void);
void pong_task(void *arg);

/* Mutex-protected snapshot of the most recent exchange -- rtt_ms is only
 * meaningful when status is PINGPONG_STATUS_OK and this node is currently
 * playing the ping role (0 otherwise, e.g. a pong reply has no round trip
 * of its own to report). Same data also published as the pingpong.status/
 * pingpong.seq/pingpong.rtt_ms metrics, for the "metrics" CLI command. */
void pingpong_task_status_read(pingpong_status_t *status, uint32_t *seq, uint32_t *rtt_ms);
