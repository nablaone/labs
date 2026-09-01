#pragma once
#include <stdint.h>

/*
 * Shared "excitement counter" -- a stand-in for later CAN-driven events
 * (each interesting bus event will eventually bump it). Every module that
 * has something worth noting (button press, CAN frame received, a
 * heartbeat tick) bumps it; led_task watches it to blink. Touched
 * concurrently by tasks that may run on either of the ESP32's two cores,
 * so access goes through a mutex rather than relying on plain reads/
 * writes being atomic.
 */

void state_init(void);

uint32_t state_counter_read(void);
void state_counter_increment(void);

uint32_t state_heartbeat_period_read(void);
void state_heartbeat_period_set(uint32_t ms);

/* Registers the "counter" CLI command. */
void state_register_cli_commands(void);
