#pragma once
#include <stdbool.h>

/*
 * Not a background task (no while(1) loop of its own) -- the TWAI driver
 * is installed once and then everything else is CLI-command-driven (self-
 * test, sniff, send). Still its own module/file, same as the task
 * modules, since it's the "hardware code" this app's design is meant to
 * grow more of (see ../../../docs/project-charter.md).
 */

/* Runs the self-test (NO_ACK loopback) once, restoring TWAI_MODE_NORMAL
 * afterward either way -- also installs the driver in the first place, so
 * this doubles as CAN's "init" step. Used both at boot and by the
 * "can loop"/"can xcvr" CLI commands. */
bool can_run_selftest(void);

/* Registers the "can" CLI command (loop/xcvr/sniff/send subcommands). */
void can_register_cli_commands(void);
