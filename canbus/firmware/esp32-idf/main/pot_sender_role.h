#pragma once

/*
 * Potentiometer bring-up exercise, sending side (see pot_collector_role.h
 * for the receiving side). Every POT_SEND_PERIOD_MS, reads the pot wired
 * to POT_GPIO (node_config.h -- also has the wiring diagram and whether
 * resistors are needed) via ADC1 and broadcasts the raw 12-bit reading
 * over CAN, publishing the same value as the pot_sender.raw metric (see
 * metrics.c) so it shows up locally too, not just on whichever board is
 * running pot_collector_role.
 */

/* Sets up the ADC1 channel, registers pot_sender.raw, and starts the
 * sending task, unconditionally -- main.c's switch on identity.h's
 * runtime role is the single place that decides whether to call this. */
void launch_pot_sender_role(void);
