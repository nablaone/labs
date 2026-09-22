#pragma once
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

/*
 * Generic key -> typed-value status/metrics registry, mutex-protected
 * since modules register/update from tasks that may run on either of the
 * ESP32's two cores. Each subsystem registers its own metrics once
 * (typically from its own _init()) and updates them as it runs. Two
 * consumers read it back: the "metrics" CLI command (registered here),
 * and display_task, which cycles the physical LCD through every
 * registered entry -- metrics_count()/metrics_get() below exist for a
 * consumer like that; nothing else needs them.
 *
 * Keys are not copied -- every caller is expected to pass a string
 * literal (static duration), e.g. "heartbeat.ticks", dotted and
 * module-prefixed to avoid collisions between modules. String *values*
 * are copied (truncated to METRIC_STRING_MAX-1) since those often come
 * from a caller's local buffer that goes out of scope.
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

/* Number of currently registered metrics -- valid indices for
 * metrics_get() below are [0, metrics_count()). Can grow between calls
 * (a module registering a new key later, e.g. ping_role.c/pong_role.c's
 * metrics only appear once this board's role is configured) -- callers
 * that walk the table on a timer, like display_task, should re-check it
 * each pass rather than caching it once. */
size_t metrics_count(void);

/* Copies the key and a human-readable value string for slot idx into the
 * caller's buffers (each null-terminated, truncated to fit). Returns
 * false (leaving the buffers untouched) if idx is out of range. */
bool metrics_get(size_t idx, char *key_out, size_t key_out_len,
		  char *value_out, size_t value_out_len);
