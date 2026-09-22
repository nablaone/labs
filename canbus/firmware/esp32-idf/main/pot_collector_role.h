#pragma once

/*
 * Potentiometer bring-up exercise, receiving side (see pot_sender_role.h
 * for the sending side). Listens for pot_sender_role's CAN broadcasts
 * and republishes the value as the pot_collector.raw metric (see
 * metrics.c) -- doesn't touch the LCD itself, same as ping_role/
 * pong_role; display_task shows it by cycling the metrics table
 * generically.
 */

/* Registers pot_collector.raw and starts the collecting task,
 * unconditionally -- main.c's switch on identity.h's runtime role is
 * the single place that decides whether to call this. */
void launch_pot_collector_role(void);
