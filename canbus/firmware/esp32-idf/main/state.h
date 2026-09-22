#pragma once
#include <stdint.h>

/*
 * Shared "excitement counter" -- a stand-in for later CAN-driven events
 * (each interesting bus event will eventually bump it). Every module that
 * has something worth noting (CAN frame received, a heartbeat tick) bumps
 * it. Touched concurrently by tasks that may run on either of the ESP32's
 * two cores, so access goes through a mutex rather than relying on plain
 * reads/writes being atomic. Also published as the "counter" metric (see
 * metrics.c) so it shows up in the "metrics" CLI table, alongside the
 * plain "counter" CLI command this module already owns.
 */

void state_init(void);

uint32_t state_counter_read(void);
void state_counter_increment(void);

/* Registers the "counter" CLI command. */
void state_register_cli_commands(void);
