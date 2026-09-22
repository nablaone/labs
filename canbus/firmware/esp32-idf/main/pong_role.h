#pragma once

/*
 * Pong side of the two-node CAN bring-up exercise (see ping_role.h for
 * the other side). Replies to every PING it sees, publishing the
 * outcome as the pong_role.status/pong_role.seq/pong_role.rtt_ms
 * metrics (see metrics.c).
 */

/* Registers this role's metrics and starts pong_task, unconditionally --
 * main.c's switch on identity.h's runtime role is the single place that
 * decides whether to call this (vs. launch_ping_role()), so this
 * function doesn't need to check the role itself. */
void launch_pong_role(void);
