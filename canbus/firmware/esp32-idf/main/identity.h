#pragma once
#include <stdbool.h>
#include <stdint.h>

/*
 * Per-unit runtime identity (node_id, role) -- unlike node_config.h's
 * compile-time NODE_ENABLE_ flags and pin settings, this is the same
 * binary flashed to every board, differentiated only by what's stored
 * in NVS.
 * Set via the "config" CLI command; survives `make flash` (which only
 * rewrites the app partition, not NVS) so a board keeps its identity
 * across rebuilds -- only `esptool erase_flash` clears it. Changing
 * role via "config set-role" reboots the board immediately (see
 * identity.c) since main.c only ever dispatches to one role's launcher
 * once, at boot -- changing node_id does not reboot, since nothing
 * reads it yet.
 */

typedef enum {
	IDENTITY_ROLE_PING = 0,
	IDENTITY_ROLE_PONG = 1,
	IDENTITY_ROLE_POT_SENDER = 2,
	IDENTITY_ROLE_POT_COLLECTOR = 3,
} identity_role_t;

/* Initializes NVS (erasing and retrying once if the partition is in a
 * state NVS can't mount -- truncated/new-format, the standard ESP-IDF
 * idiom) and loads any previously saved node_id/role. Safe to call on a
 * never-configured board -- identity_node_id_read()/identity_role_read()
 * just report "unset". Must run before anything that needs this node's
 * identity (e.g. which CAN IDs it sends/listens on). */
void identity_init(void);

/* True once both node_id and role have been set, this boot or a
 * previous one (both persist). */
bool identity_is_configured(void);

/* Each returns false ("unset") until the matching _set() has been called,
 * this boot or a previous one. */
bool identity_node_id_read(uint8_t *out_id);
bool identity_node_id_set(uint8_t id);

bool identity_role_read(identity_role_t *out_role);
/* Persists to NVS, updates the cached value, then reboots the board
 * (esp_restart()) -- see identity.c. Only returns (false) if the NVS
 * write itself failed. */
bool identity_role_set(identity_role_t role);

/* Registers the "config" CLI command (show/set-id/set-role). */
void identity_register_cli_commands(void);
