#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_console.h"
#include "esp_log.h"
#include "esp_system.h"
#include "nvs.h"
#include "nvs_flash.h"

#include "identity.h"
#include "metrics.h"

static const char *TAG = "identity";

#define NVS_NAMESPACE   "identity"
#define NVS_KEY_NODE_ID "node_id"
#define NVS_KEY_ROLE    "role"

static nvs_handle_t nvs;

/* Cached copies so reads don't hit flash every call -- written back to
 * NVS synchronously in every _set(), so cache and flash never drift. */
static bool node_id_known;
static uint8_t node_id_value;
static bool role_known;
static identity_role_t role_value;

static const char *role_name(identity_role_t role)
{
	switch (role) {
	case IDENTITY_ROLE_PING:
		return "ping";
	case IDENTITY_ROLE_PONG:
		return "pong";
	default:
		return "<unknown>";
	}
}

void identity_init(void)
{
	esp_err_t err = nvs_flash_init();
	if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
		ESP_ERROR_CHECK(nvs_flash_erase());
		err = nvs_flash_init();
	}
	ESP_ERROR_CHECK(err);

	ESP_ERROR_CHECK(nvs_open(NVS_NAMESPACE, NVS_READWRITE, &nvs));

	uint8_t id;
	node_id_known = nvs_get_u8(nvs, NVS_KEY_NODE_ID, &id) == ESP_OK;
	if (node_id_known) {
		node_id_value = id;
	}

	uint8_t role;
	role_known = nvs_get_u8(nvs, NVS_KEY_ROLE, &role) == ESP_OK;
	if (role_known) {
		role_value = (identity_role_t)role;
	}

	if (node_id_known && role_known) {
		ESP_LOGI(TAG, "node_id=%u role=%s", node_id_value, role_name(role_value));
	} else {
		ESP_LOGW(TAG, "unconfigured -- use the 'config' CLI command to set node_id/role");
	}

	metrics_register_string("identity.role", role_known ? role_name(role_value) : "unset");
}

bool identity_is_configured(void)
{
	return node_id_known && role_known;
}

bool identity_node_id_read(uint8_t *out_id)
{
	if (!node_id_known) {
		return false;
	}
	*out_id = node_id_value;
	return true;
}

bool identity_node_id_set(uint8_t id)
{
	if (nvs_set_u8(nvs, NVS_KEY_NODE_ID, id) != ESP_OK || nvs_commit(nvs) != ESP_OK) {
		return false;
	}
	node_id_value = id;
	node_id_known = true;
	return true;
}

bool identity_role_read(identity_role_t *out_role)
{
	if (!role_known) {
		return false;
	}
	*out_role = role_value;
	return true;
}

/* Reboots on success (see identity.h) -- main.c only picks ping_task/
 * pong_task once, at boot, so there'd be no other way for a role change
 * to actually take effect. fflush()+a short delay give the console time
 * to send the caller's own "role set to ..." confirmation before the
 * UART goes down. */
bool identity_role_set(identity_role_t role)
{
	if (nvs_set_u8(nvs, NVS_KEY_ROLE, (uint8_t)role) != ESP_OK || nvs_commit(nvs) != ESP_OK) {
		return false;
	}
	role_value = role;
	role_known = true;
	metrics_set_string("identity.role", role_name(role));

	fflush(stdout);
	vTaskDelay(pdMS_TO_TICKS(100));
	esp_restart();
}

static int cmd_config_show(void)
{
	uint8_t id;
	if (identity_node_id_read(&id)) {
		printf("node_id: %u\n", id);
	} else {
		printf("node_id: unset\n");
	}

	identity_role_t role;
	if (identity_role_read(&role)) {
		printf("role:    %s\n", role_name(role));
	} else {
		printf("role:    unset\n");
	}

	return 0;
}

static int cmd_config_set_id(const char *arg)
{
	char *end;
	long n = strtol(arg, &end, 10);
	if (*end != '\0' || n < 0 || n > 255) {
		printf("bad node id '%s' (expected 0-255)\n", arg);
		return 1;
	}

	if (!identity_node_id_set((uint8_t)n)) {
		printf("failed to save node id\n");
		return 1;
	}

	printf("node_id set to %ld\n", n);
	return 0;
}

static int cmd_config_set_role(const char *arg)
{
	identity_role_t role;
	if (strcmp(arg, "ping") == 0) {
		role = IDENTITY_ROLE_PING;
	} else if (strcmp(arg, "pong") == 0) {
		role = IDENTITY_ROLE_PONG;
	} else {
		printf("bad role '%s' (expected 'ping' or 'pong')\n", arg);
		return 1;
	}

	printf("saving role '%s'...\n", arg);
	if (!identity_role_set(role)) {
		printf("failed to save role\n");
		return 1;
	}

	return 0; /* unreachable: identity_role_set() reboots on success */
}

static int cmd_config(int argc, char **argv)
{
	if (argc < 2) {
		printf("usage: config <show|set-id <n>|set-role <ping|pong>>\n");
		return 1;
	}

	if (strcmp(argv[1], "show") == 0) {
		return cmd_config_show();
	} else if (strcmp(argv[1], "set-id") == 0) {
		if (argc < 3) {
			printf("usage: config set-id <n>\n");
			return 1;
		}
		return cmd_config_set_id(argv[2]);
	} else if (strcmp(argv[1], "set-role") == 0) {
		if (argc < 3) {
			printf("usage: config set-role <ping|pong>\n");
			return 1;
		}
		return cmd_config_set_role(argv[2]);
	}

	printf("unknown config subcommand: '%s'\n", argv[1]);
	return 1;
}

void identity_register_cli_commands(void)
{
	const esp_console_cmd_t config_cmd = {
		.command = "config",
		.help = "Node identity: show | set-id <n> | set-role <ping|pong> (reboots)",
		.func = &cmd_config,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&config_cmd));
}
