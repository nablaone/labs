#pragma once
#include <stdbool.h>
#include <stdint.h>

/*
 * Per-unit runtime identity (node_id, mode) -- unlike node_config.h's
 * compile-time NODE_ENABLE_ flags and pin settings, this is the same
 * binary flashed to every board, differentiated only by what's stored
 * in NVS.
 * Set via the "config" CLI command; survives `make flash` (which only
 * rewrites the app partition, not NVS) so a board keeps its identity
 * across rebuilds -- only `esptool erase_flash` clears it.
 */

typedef enum {
	IDENTITY_MODE_PING = 0,
	IDENTITY_MODE_PONG = 1,
} identity_mode_t;

/* Initializes NVS (erasing and retrying once if the partition is in a
 * state NVS can't mount -- truncated/new-format, the standard ESP-IDF
 * idiom) and loads any previously saved node_id/mode. Safe to call on a
 * never-configured board -- identity_node_id_read()/identity_mode_read()
 * just report "unset". Must run before anything that needs this node's
 * identity (e.g. which CAN IDs it sends/listens on). */
void identity_init(void);

/* True once both node_id and mode have been set, this boot or a
 * previous one (both persist). */
bool identity_is_configured(void);

/* Each returns false ("unset") until the matching _set() has been called,
 * this boot or a previous one. */
bool identity_node_id_read(uint8_t *out_id);
bool identity_node_id_set(uint8_t id);

bool identity_mode_read(identity_mode_t *out_mode);
bool identity_mode_set(identity_mode_t mode);

/* Registers the "config" CLI command (show/set-id/set-mode). */
void identity_register_cli_commands(void);
