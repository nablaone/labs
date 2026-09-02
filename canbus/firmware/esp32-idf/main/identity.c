#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "esp_console.h"
#include "esp_log.h"
#include "nvs.h"
#include "nvs_flash.h"

#include "identity.h"

static const char *TAG = "identity";

#define NVS_NAMESPACE   "identity"
#define NVS_KEY_NODE_ID "node_id"
#define NVS_KEY_MODE    "mode"

static nvs_handle_t nvs;

/* Cached copies so reads don't hit flash every call -- written back to
 * NVS synchronously in every _set(), so cache and flash never drift. */
static bool node_id_known;
static uint8_t node_id_value;
static bool mode_known;
static identity_mode_t mode_value;

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

	uint8_t mode;
	mode_known = nvs_get_u8(nvs, NVS_KEY_MODE, &mode) == ESP_OK;
	if (mode_known) {
		mode_value = (identity_mode_t)mode;
	}

	if (node_id_known && mode_known) {
		ESP_LOGI(TAG, "node_id=%u mode=%s", node_id_value,
			 mode_value == IDENTITY_MODE_PING ? "ping" : "pong");
	} else {
		ESP_LOGW(TAG, "unconfigured -- use the 'config' CLI command to set node_id/mode");
	}
}

bool identity_is_configured(void)
{
	return node_id_known && mode_known;
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

bool identity_mode_read(identity_mode_t *out_mode)
{
	if (!mode_known) {
		return false;
	}
	*out_mode = mode_value;
	return true;
}

bool identity_mode_set(identity_mode_t mode)
{
	if (nvs_set_u8(nvs, NVS_KEY_MODE, (uint8_t)mode) != ESP_OK || nvs_commit(nvs) != ESP_OK) {
		return false;
	}
	mode_value = mode;
	mode_known = true;
	return true;
}

static const char *mode_name(identity_mode_t mode)
{
	return mode == IDENTITY_MODE_PING ? "ping" : "pong";
}

static int cmd_config_show(void)
{
	uint8_t id;
	if (identity_node_id_read(&id)) {
		printf("node_id: %u\n", id);
	} else {
		printf("node_id: unset\n");
	}

	identity_mode_t mode;
	if (identity_mode_read(&mode)) {
		printf("mode:    %s\n", mode_name(mode));
	} else {
		printf("mode:    unset\n");
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

static int cmd_config_set_mode(const char *arg)
{
	identity_mode_t mode;
	if (strcmp(arg, "ping") == 0) {
		mode = IDENTITY_MODE_PING;
	} else if (strcmp(arg, "pong") == 0) {
		mode = IDENTITY_MODE_PONG;
	} else {
		printf("bad mode '%s' (expected 'ping' or 'pong')\n", arg);
		return 1;
	}

	if (!identity_mode_set(mode)) {
		printf("failed to save mode\n");
		return 1;
	}

	printf("mode set to %s\n", arg);
	return 0;
}

static int cmd_config(int argc, char **argv)
{
	if (argc < 2) {
		printf("usage: config <show|set-id <n>|set-mode <ping|pong>>\n");
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
	} else if (strcmp(argv[1], "set-mode") == 0) {
		if (argc < 3) {
			printf("usage: config set-mode <ping|pong>\n");
			return 1;
		}
		return cmd_config_set_mode(argv[2]);
	}

	printf("unknown config subcommand: '%s'\n", argv[1]);
	return 1;
}

void identity_register_cli_commands(void)
{
	const esp_console_cmd_t config_cmd = {
		.command = "config",
		.help = "Node identity: show | set-id <n> | set-mode <ping|pong>",
		.func = &cmd_config,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&config_cmd));
}
