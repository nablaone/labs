#pragma once

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
 * never touches the LCD itself. display_task doesn't know this module
 * exists either: it just cycles through every metrics.c entry, and this
 * module's pingpong.status/pingpong.seq/pingpong.rtt_ms (registered in
 * ping_task_init()/pong_task_init(), updated in status_set()) are three
 * of those entries, same as any other module's.
 */

typedef enum {
	PINGPONG_STATUS_NONE = 0,    /* no exchange yet this boot */
	PINGPONG_STATUS_OK,          /* ping: got a matching pong. pong: replied to a ping */
	PINGPONG_STATUS_TIMEOUT,     /* ping only: no matching pong within PING_TIMEOUT_MS */
} pingpong_status_t;

/* Sends a PING every PING_PERIOD_MS and waits for the matching PONG.
 * ping_task_init() registers the pingpong.status/pingpong.seq/
 * pingpong.rtt_ms metrics (see metrics.c) -- call whichever of this or
 * pong_task_init() matches the task main.c is about to start; either one
 * is enough, both do the same thing. */
void ping_task_init(void);
void ping_task(void *arg);

/* Replies to every PING it sees. pong_task_init() is ping_task_init()'s
 * counterpart -- see above. */
void pong_task_init(void);
void pong_task(void *arg);
