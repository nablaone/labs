#include <stdio.h>
#include <string.h>
#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include "esp_console.h"
#include "esp_log.h"

#include "metrics.h"

static const char *TAG = "metrics";

#define METRICS_MAX 32

typedef struct {
	const char *key;
	metric_type_t type;
	union {
		bool b;
		int32_t i;
		float f;
		char s[METRIC_STRING_MAX];
	} value;
} metric_t;

static metric_t metrics[METRICS_MAX];
static size_t num_metrics;
static SemaphoreHandle_t metrics_mutex;

void metrics_init(void)
{
	metrics_mutex = xSemaphoreCreateMutex();
}

/* Linear search -- table is small (METRICS_MAX) and lookups are rare
 * (register once per key, set occasionally from task loops), so nothing
 * fancier is worth it. Caller holds metrics_mutex. */
static metric_t *find(const char *key)
{
	for (size_t i = 0; i < num_metrics; i++) {
		if (strcmp(metrics[i].key, key) == 0) {
			return &metrics[i];
		}
	}
	return NULL;
}

/* Appends a new slot for key/type. Caller holds metrics_mutex. Returns
 * NULL (having logged why) if the table is full. */
static metric_t *add_slot(const char *key, metric_type_t type)
{
	if (num_metrics == METRICS_MAX) {
		ESP_LOGE(TAG, "table full (%d) -- '%s' not registered", METRICS_MAX, key);
		return NULL;
	}
	metric_t *m = &metrics[num_metrics++];
	m->key = key;
	m->type = type;
	return m;
}

/* Finds key, creating it if new; warns and switches type if it already
 * existed under a different type. Caller holds metrics_mutex. May return
 * NULL (table full on a new key) -- callers must check. */
static metric_t *find_or_register(const char *key, metric_type_t type)
{
	metric_t *m = find(key);
	if (m == NULL) {
		return add_slot(key, type);
	}
	if (m->type != type) {
		ESP_LOGW(TAG, "'%s' re-registered with a different type", key);
		m->type = type;
	}
	return m;
}

void metrics_register_bool(const char *key, bool initial)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find_or_register(key, METRIC_TYPE_BOOL);
	if (m != NULL) {
		m->value.b = initial;
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_register_int(const char *key, int32_t initial)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find_or_register(key, METRIC_TYPE_INT);
	if (m != NULL) {
		m->value.i = initial;
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_register_float(const char *key, float initial)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find_or_register(key, METRIC_TYPE_FLOAT);
	if (m != NULL) {
		m->value.f = initial;
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_register_string(const char *key, const char *initial)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find_or_register(key, METRIC_TYPE_STRING);
	if (m != NULL) {
		snprintf(m->value.s, sizeof(m->value.s), "%s", initial);
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_set_bool(const char *key, bool value)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find(key);
	if (m == NULL) {
		ESP_LOGE(TAG, "'%s' not registered", key);
	} else if (m->type != METRIC_TYPE_BOOL) {
		ESP_LOGE(TAG, "'%s' is not a bool metric", key);
	} else {
		m->value.b = value;
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_set_int(const char *key, int32_t value)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find(key);
	if (m == NULL) {
		ESP_LOGE(TAG, "'%s' not registered", key);
	} else if (m->type != METRIC_TYPE_INT) {
		ESP_LOGE(TAG, "'%s' is not an int metric", key);
	} else {
		m->value.i = value;
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_set_float(const char *key, float value)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find(key);
	if (m == NULL) {
		ESP_LOGE(TAG, "'%s' not registered", key);
	} else if (m->type != METRIC_TYPE_FLOAT) {
		ESP_LOGE(TAG, "'%s' is not a float metric", key);
	} else {
		m->value.f = value;
	}
	xSemaphoreGive(metrics_mutex);
}

void metrics_set_string(const char *key, const char *value)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	metric_t *m = find(key);
	if (m == NULL) {
		ESP_LOGE(TAG, "'%s' not registered", key);
	} else if (m->type != METRIC_TYPE_STRING) {
		ESP_LOGE(TAG, "'%s' is not a string metric", key);
	} else {
		snprintf(m->value.s, sizeof(m->value.s), "%s", value);
	}
	xSemaphoreGive(metrics_mutex);
}

static const char *type_name(metric_type_t type)
{
	switch (type) {
	case METRIC_TYPE_BOOL:   return "bool";
	case METRIC_TYPE_INT:    return "int";
	case METRIC_TYPE_FLOAT:  return "float";
	case METRIC_TYPE_STRING: return "string";
	default:                 return "?";
	}
}

/* Shared by cmd_metrics() and metrics_get() so there's exactly one place
 * that knows how to turn each value type into text. Caller holds
 * metrics_mutex. */
static void format_value(const metric_t *m, char *out, size_t out_len)
{
	switch (m->type) {
	case METRIC_TYPE_BOOL:
		snprintf(out, out_len, "%s", m->value.b ? "true" : "false");
		break;
	case METRIC_TYPE_INT:
		snprintf(out, out_len, "%" PRId32, m->value.i);
		break;
	case METRIC_TYPE_FLOAT:
		snprintf(out, out_len, "%g", m->value.f);
		break;
	case METRIC_TYPE_STRING:
		snprintf(out, out_len, "%s", m->value.s);
		break;
	}
}

static int cmd_metrics(int argc, char **argv)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	if (num_metrics == 0) {
		printf("(no metrics registered)\n");
	} else {
		printf("%-20s %-7s %s\n", "key", "type", "value");
		for (size_t i = 0; i < num_metrics; i++) {
			char value[32];
			format_value(&metrics[i], value, sizeof(value));
			printf("%-20s %-7s %s\n", metrics[i].key, type_name(metrics[i].type), value);
		}
	}
	xSemaphoreGive(metrics_mutex);
	return 0;
}

void metrics_register_cli_commands(void)
{
	const esp_console_cmd_t metrics_cmd = {
		.command = "metrics",
		.help = "Show all registered metrics",
		.func = &cmd_metrics,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&metrics_cmd));
}

size_t metrics_count(void)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	size_t count = num_metrics;
	xSemaphoreGive(metrics_mutex);
	return count;
}

bool metrics_get(size_t idx, char *key_out, size_t key_out_len,
		  char *value_out, size_t value_out_len)
{
	xSemaphoreTake(metrics_mutex, portMAX_DELAY);
	if (idx >= num_metrics) {
		xSemaphoreGive(metrics_mutex);
		return false;
	}
	snprintf(key_out, key_out_len, "%s", metrics[idx].key);
	format_value(&metrics[idx], value_out, value_out_len);
	xSemaphoreGive(metrics_mutex);
	return true;
}
