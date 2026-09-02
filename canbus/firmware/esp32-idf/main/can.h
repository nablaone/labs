#pragma once
#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

#include "freertos/FreeRTOS.h"
#include "driver/twai.h"

/*
 * The TWAI driver is installed once (by can_run_selftest(), which doubles
 * as init) and then most of this module is CLI-command-driven (self-test,
 * sniff, send) or called directly by other modules (can_send()). The one
 * exception is can_rx_task(): once started, it's the sole reader of
 * twai_receive() and fans every frame out to one software queue that any
 * consumer -- the "can sniff" CLI command, pingpong_task -- drains via
 * can_receive(). Still one file/module, same as the task modules, since
 * it's the "hardware code" this app's design is meant to grow more of
 * (see ../../../docs/project-charter.md).
 */

/* Runs the self-test (NO_ACK loopback) once, restoring TWAI_MODE_NORMAL
 * afterward either way -- also installs the driver in the first place, so
 * this doubles as CAN's "init" step. Used both at boot and by the
 * "can loop"/"can xcvr" CLI commands. */
bool can_run_selftest(void);

/* Transmits one standard-frame CAN message (len must be <= 8). Returns
 * false and logs the reason on failure (transceiver/bus trouble) --
 * caller doesn't need its own error handling beyond checking the result.
 * Shared by the "can send" CLI command and other modules that want to
 * put something on the bus (e.g. display_task's periodic counter
 * broadcast, button_task's press-event broadcast). */
bool can_send(uint32_t id, const uint8_t *data, size_t len);

/* Convenience wrapper for the common case of a single little-endian
 * 32-bit value payload (../../../docs/can-message-spec.md's stated byte
 * convention) -- e.g. broadcasting the excitement counter. */
bool can_send_u32(uint32_t id, uint32_t value);

/* Like can_send()/can_send_u32(), but returns immediately (0 timeout on
 * twai_transmit()) instead of waiting up to 1s for room in the driver's
 * TX queue. Note this is about queue space, not the frame's bus-level
 * ACK -- the CAN controller itself decides ACK/retry behavior regardless
 * of which of these a caller uses; this only controls whether *this*
 * call blocks if the queue's already backed up. Use for a caller that
 * fires often (button_task's press-event broadcast, once per 100ms poll
 * while held) and shouldn't stall its own loop if the bus is congested
 * or has no listener. */
bool can_send_nowait(uint32_t id, const uint8_t *data, size_t len);
bool can_send_u32_nowait(uint32_t id, uint32_t value);

/* Registers the "can" CLI command (loop/xcvr/sniff/send subcommands). */
void can_register_cli_commands(void);

/* Background task: blocks in twai_receive() and pushes every frame onto
 * the shared RX queue can_receive() drains. Start after can_run_selftest()
 * (which creates that queue) has run at least once. Don't call
 * twai_receive() directly anywhere else once this is running -- it would
 * compete with this task for frames straight off the driver, before they
 * ever reach the software queue. */
void can_rx_task(void *arg);

/* Blocks up to `timeout` for the next frame can_rx_task received. Shared
 * by the "can sniff" CLI command and pingpong_task -- running "can sniff"
 * while pingpong is active will steal frames from it (a manual diagnostic
 * command competing with a task, not meant to run both at once).
 * Returns false on timeout. */
bool can_receive(twai_message_t *msg, TickType_t timeout);
