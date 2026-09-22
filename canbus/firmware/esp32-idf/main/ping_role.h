#pragma once

/*
 * Ping side of the two-node CAN bring-up exercise (see pong_role.h for
 * the other side). Sends a PING every PING_PERIOD_MS and waits for the
 * matching PONG, publishing the outcome as the pingpong.status/
 * pingpong.seq/pingpong.rtt_ms metrics (see metrics.c).
 */

/* Registers this role's metrics and starts ping_task, unconditionally --
 * main.c's switch on identity.h's runtime role is the single place that
 * decides whether to call this (vs. launch_pong_role()), so this
 * function doesn't need to check the role itself. */
void launch_ping_role(void);
