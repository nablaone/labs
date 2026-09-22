#pragma once
#include <stdbool.h>
#include <stdint.h>

/*
 * Generic key -> typed-value status/metrics registry, mutex-protected
 * since modules register/update from tasks that may run on either of the
 * ESP32's two cores. Each subsystem registers its own metrics once
 * (typically from its own _init()) and updates them as it runs; the
 * "metrics" CLI command (registered here) is the only consumer -- it
 * dumps the whole table, so no public read/getter API is needed.
 *
 * Keys are not copied -- every caller is expected to pass a string
 * literal (static duration), e.g. "heartbeat.ticks", dotted and
 * module-prefixed to avoid collisions between modules. String *values*
 * are copied (truncated to METRIC_STRING_MAX-1) since those often come
 * from a caller's local buffer that goes out of scope.
 *
 * Phase 1 (this module): the registry + CLI command only, nothing else
 * wired to it yet. Migrating existing per-module getters (state.c's
 * counter, identity.c's role/node_id, pingpong_task's status/seq/rtt) to
 * also publish here is a deliberately separate follow-up, not part of
 * this change -- display_task's LCD tabs keep reading those directly,
 * since that's a different, latency-sensitive consumer this registry
 * isn't meant to replace.
 */

#define METRIC_STRING_MAX 20

typedef enum {
	METRIC_TYPE_BOOL,
	METRIC_TYPE_INT,
	METRIC_TYPE_FLOAT,
	METRIC_TYPE_STRING,
} metric_type_t;

void metrics_init(void);

/* Registers key with an initial value -- idempotent: registering an
 * already-registered key just updates its value (logging a warning if
 * the type doesn't match what it was first registered as, then using
 * the new type). Logs an error and does nothing if the table is full. */
void metrics_register_bool(const char *key, bool initial);
void metrics_register_int(const char *key, int32_t initial);
void metrics_register_float(const char *key, float initial);
void metrics_register_string(const char *key, const char *initial);

/* Updates an already-registered key. Logs an error and does nothing if
 * key isn't registered, or was registered as a different type. */
void metrics_set_bool(const char *key, bool value);
void metrics_set_int(const char *key, int32_t value);
void metrics_set_float(const char *key, float value);
void metrics_set_string(const char *key, const char *value);

/* Registers the "metrics" CLI command. */
void metrics_register_cli_commands(void);
