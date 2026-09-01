#pragma once
#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

/*
 * Not a background task (no while(1) loop of its own) -- the TWAI driver
 * is installed once and then everything else is CLI-command-driven (self-
 * test, sniff, send) or called directly by other modules (can_send()).
 * Still its own module/file, same as the task modules, since it's the
 * "hardware code" this app's design is meant to grow more of (see
 * ../../../docs/project-charter.md).
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

/* Registers the "can" CLI command (loop/xcvr/sniff/send subcommands). */
void can_register_cli_commands(void);
