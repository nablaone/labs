#include <stdio.h>
#include <string.h>
#include <stdbool.h>
#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_console.h"
#include "nvs_flash.h"
#include "nimble/nimble_port.h"
#include "nimble/nimble_port_freertos.h"
#include "host/ble_hs.h"
#include "host/util/util.h"
#include "services/gap/ble_svc_gap.h"
#include "services/gatt/ble_svc_gatt.h"

#include "dcc.h"
#include "ble_link.h"

static const char *TAG = "ble";

#define BLE_DEVICE_NAME "DCC-Sniffer"

/* See ble_link.h for the record format. */
#define RECORD_HEADER 5
#define RECORD_MAX    (RECORD_HEADER + DCC_MAX_BYTES)

/* Longest a packet waits in a half-full batch before being sent. */
#define BLE_BATCH_MS 50

/* Largest notification payload we build: MTU 247 minus the 3-byte ATT
 * header, the size Chrome typically negotiates. Smaller MTUs just get
 * smaller batches. */
#define NOTIFY_MAX 244

/* UUIDs from ble_link.h, byte order reversed (NimBLE stores 128-bit
 * UUIDs little-endian). */
static const ble_uuid128_t svc_uuid = BLE_UUID128_INIT(
	0x5f, 0x4e, 0x3d, 0x2c, 0x1b, 0x5a, 0x3e, 0x9f,
	0x8a, 0x4c, 0x2e, 0x6b, 0x01, 0x00, 0xcc, 0xd0);
static const ble_uuid128_t packets_uuid = BLE_UUID128_INIT(
	0x5f, 0x4e, 0x3d, 0x2c, 0x1b, 0x5a, 0x3e, 0x9f,
	0x8a, 0x4c, 0x2e, 0x6b, 0x02, 0x00, 0xcc, 0xd0);

static uint16_t packets_handle;
static uint8_t own_addr_type;

/* Written by the NimBLE host task (GAP events), read by
 * ble_link_tx_task and the CLI. */
static volatile uint16_t conn_handle = BLE_HS_CONN_HANDLE_NONE;
static volatile bool subscribed;
static volatile uint16_t mtu = BLE_ATT_MTU_DFLT;

static volatile struct {
	uint32_t connects;
	uint32_t notifications;
	uint32_t packets;
	uint32_t dropped;  /* notify failed (BLE buffers full) */
} stats;

/* Notify-only characteristic: there's nothing to read or write, so
 * NimBLE never actually calls this. */
static int gatt_access(uint16_t conn, uint16_t attr,
		       struct ble_gatt_access_ctxt *ctxt, void *arg)
{
	return BLE_ATT_ERR_UNLIKELY;
}

static const struct ble_gatt_svc_def gatt_svcs[] = {
	{
		.type = BLE_GATT_SVC_TYPE_PRIMARY,
		.uuid = &svc_uuid.u,
		.characteristics = (struct ble_gatt_chr_def[]) {
			{
				.uuid = &packets_uuid.u,
				.access_cb = gatt_access,
				.flags = BLE_GATT_CHR_F_NOTIFY,
				.val_handle = &packets_handle,
			},
			{ 0 },
		},
	},
	{ 0 },
};

static int gap_event(struct ble_gap_event *event, void *arg);

/* The 128-bit service UUID (so Web Bluetooth's service filter finds us)
 * and the name don't both fit the 31-byte advertisement, so the name
 * goes in the scan response. */
static void advertise(void)
{
	struct ble_hs_adv_fields fields = {
		.flags = BLE_HS_ADV_F_DISC_GEN | BLE_HS_ADV_F_BREDR_UNSUP,
		.uuids128 = &svc_uuid,
		.num_uuids128 = 1,
		.uuids128_is_complete = 1,
	};
	int rc = ble_gap_adv_set_fields(&fields);
	if (rc != 0) {
		ESP_LOGE(TAG, "adv fields: %d", rc);
		return;
	}

	struct ble_hs_adv_fields rsp = {
		.name = (const uint8_t *)BLE_DEVICE_NAME,
		.name_len = sizeof(BLE_DEVICE_NAME) - 1,
		.name_is_complete = 1,
	};
	rc = ble_gap_adv_rsp_set_fields(&rsp);
	if (rc != 0) {
		ESP_LOGE(TAG, "scan response fields: %d", rc);
		return;
	}

	struct ble_gap_adv_params params = {
		.conn_mode = BLE_GAP_CONN_MODE_UND,
		.disc_mode = BLE_GAP_DISC_MODE_GEN,
	};
	rc = ble_gap_adv_start(own_addr_type, NULL, BLE_HS_FOREVER, &params,
			       gap_event, NULL);
	if (rc != 0) {
		ESP_LOGE(TAG, "adv start: %d", rc);
	}
}

static int gap_event(struct ble_gap_event *event, void *arg)
{
	switch (event->type) {
	case BLE_GAP_EVENT_CONNECT:
		if (event->connect.status == 0) {
			conn_handle = event->connect.conn_handle;
			stats.connects++;
			ESP_LOGI(TAG, "client connected");
		} else {
			advertise();
		}
		break;

	case BLE_GAP_EVENT_DISCONNECT:
		ESP_LOGI(TAG, "client disconnected (reason 0x%x)", event->disconnect.reason);
		conn_handle = BLE_HS_CONN_HANDLE_NONE;
		subscribed = false;
		mtu = BLE_ATT_MTU_DFLT;
		advertise();
		break;

	case BLE_GAP_EVENT_SUBSCRIBE:
		if (event->subscribe.attr_handle == packets_handle) {
			subscribed = event->subscribe.cur_notify;
			ESP_LOGI(TAG, "notifications %s", subscribed ? "on" : "off");
		}
		break;

	case BLE_GAP_EVENT_MTU:
		mtu = event->mtu.value;
		break;

	case BLE_GAP_EVENT_ADV_COMPLETE:
		advertise();
		break;
	}
	return 0;
}

static void on_sync(void)
{
	ble_hs_util_ensure_addr(0);
	ble_hs_id_infer_auto(0, &own_addr_type);
	advertise();
	ESP_LOGI(TAG, "advertising as %s", BLE_DEVICE_NAME);
}

static void on_reset(int reason)
{
	ESP_LOGW(TAG, "host reset, reason %d", reason);
}

static void host_task(void *arg)
{
	nimble_port_run();  /* returns only on nimble_port_stop() */
	nimble_port_freertos_deinit();
}

void ble_link_init(void)
{
	/* The BLE controller keeps its PHY calibration data in NVS. */
	esp_err_t err = nvs_flash_init();
	if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
		ESP_ERROR_CHECK(nvs_flash_erase());
		err = nvs_flash_init();
	}
	ESP_ERROR_CHECK(err);

	ESP_ERROR_CHECK(nimble_port_init());
	ble_hs_cfg.sync_cb = on_sync;
	ble_hs_cfg.reset_cb = on_reset;

	ble_svc_gap_init();
	ble_svc_gatt_init();
	ESP_ERROR_CHECK(ble_gatts_count_cfg(gatt_svcs) == 0 ? ESP_OK : ESP_FAIL);
	ESP_ERROR_CHECK(ble_gatts_add_svcs(gatt_svcs) == 0 ? ESP_OK : ESP_FAIL);
	ble_svc_gap_device_name_set(BLE_DEVICE_NAME);

	nimble_port_freertos_init(host_task);
}

/* ---- packet streaming ---- */

static uint8_t batch[NOTIFY_MAX];
static size_t batch_len;
static unsigned batch_count;

static void flush(void)
{
	if (batch_len == 0) {
		return;
	}

	/* ble_gatts_notify_custom() takes ownership of the mbuf, freeing it
	 * on failure too. */
	struct os_mbuf *om = ble_hs_mbuf_from_flat(batch, batch_len);
	int rc = om ? ble_gatts_notify_custom(conn_handle, packets_handle, om)
		    : BLE_HS_ENOMEM;
	if (rc == 0) {
		stats.notifications++;
		stats.packets += batch_count;
	} else {
		stats.dropped += batch_count;
	}

	batch_len = 0;
	batch_count = 0;
}

void ble_link_tx_task(void *arg)
{
	bool was_subscribed = false;
	TickType_t batch_start = 0;

	while (1) {
		if (!subscribed) {
			/* Leave the queue to "dcc sniff" while nobody's
			 * listening over BLE. */
			was_subscribed = false;
			batch_len = 0;
			batch_count = 0;
			vTaskDelay(pdMS_TO_TICKS(100));
			continue;
		}

		if (!was_subscribed) {
			/* Drop the backlog that piled up while unsubscribed --
			 * the client wants live traffic. */
			dcc_packet_t stale;
			while (dcc_receive(&stale, 0)) {
			}
			was_subscribed = true;
		}

		size_t cap = mtu - 3;
		if (cap > sizeof(batch)) {
			cap = sizeof(batch);
		}

		dcc_packet_t p;
		if (dcc_receive(&p, pdMS_TO_TICKS(BLE_BATCH_MS))) {
			size_t rec = RECORD_HEADER + p.len;
			if (batch_len + rec > cap) {
				flush();
			}
			if (batch_len == 0) {
				batch_start = xTaskGetTickCount();
			}

			uint32_t t_ms = (uint32_t)(p.time_us / 1000);
			uint8_t *r = batch + batch_len;
			r[0] = p.len;
			r[1] = (uint8_t)t_ms;
			r[2] = (uint8_t)(t_ms >> 8);
			r[3] = (uint8_t)(t_ms >> 16);
			r[4] = (uint8_t)(t_ms >> 24);
			memcpy(r + RECORD_HEADER, p.data, p.len);
			batch_len += rec;
			batch_count++;
		}

		if (batch_len > 0 &&
		    (batch_len + RECORD_MAX > cap ||
		     xTaskGetTickCount() - batch_start >= pdMS_TO_TICKS(BLE_BATCH_MS))) {
			flush();
		}
	}
}

/* ---- CLI ---- */

static int cmd_ble(int argc, char **argv)
{
	bool connected = conn_handle != BLE_HS_CONN_HANDLE_NONE;

	printf("name           %s\n", BLE_DEVICE_NAME);
	printf("state          %s\n",
	       !connected ? "advertising" : subscribed ? "streaming" : "connected (not subscribed)");
	printf("mtu            %u\n", (unsigned)mtu);
	printf("connects       %" PRIu32 "\n", stats.connects);
	printf("notifications  %" PRIu32 "\n", stats.notifications);
	printf("packets sent   %" PRIu32 "\n", stats.packets);
	printf("packets lost   %" PRIu32 "\n", stats.dropped);
	return 0;
}

void ble_link_register_cli_commands(void)
{
	const esp_console_cmd_t ble_cmd = {
		.command = "ble",
		.help = "BLE link state and counters (streaming to the web page)",
		.func = &cmd_ble,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&ble_cmd));
}
