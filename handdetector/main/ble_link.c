#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <inttypes.h>
#include <stdarg.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/queue.h"
#include "esp_log.h"
#include "esp_console.h"
#include "esp_heap_caps.h"
#include "esp_idf_version.h"
#include "esp_rom_crc.h"
#include "esp_timer.h"
#include "nvs_flash.h"
#include "nimble/nimble_port.h"
#include "nimble/nimble_port_freertos.h"
#include "host/ble_hs.h"
#include "host/util/util.h"
#include "services/gap/ble_svc_gap.h"
#include "services/gatt/ble_svc_gatt.h"

#include "node_config.h"
#include "camera.h"
#include "detector.h"

/* NimBLE host's count of free controller ACL buffers (ble_hs_hci.c).
 * 0 means everything handed down is still waiting to go over the air. */
extern uint16_t ble_hs_hci_avail_pkts;
#include "ble_link.h"

static const char *TAG = "ble";

#define BLE_DEVICE_NAME "HandDetector"

#define REC_HEADER 0x01
#define REC_DATA   0x02
#define HEADER_LEN 22
#define DATA_HDR   7

#define CMD_MAX 64

/* UUIDs from ble_link.h, byte order reversed (NimBLE stores 128-bit
 * UUIDs little-endian). Only the 13th byte differs between them. */
#define HD_UUID(n) BLE_UUID128_INIT(                       \
	0x5f, 0x4e, 0x3d, 0x2c, 0x1b, 0x5a, 0x3e, 0x9f,    \
	0x8a, 0x4c, 0x2e, 0x6b, (n), 0x00, 0x44, 0x48)

static const ble_uuid128_t svc_uuid = HD_UUID(0x01);
static const ble_uuid128_t ctrl_uuid = HD_UUID(0x02);
static const ble_uuid128_t event_uuid = HD_UUID(0x03);
static const ble_uuid128_t info_uuid = HD_UUID(0x04);
static const ble_uuid128_t frame_uuid = HD_UUID(0x05);

static uint16_t event_handle, info_handle, frame_handle;
static uint8_t own_addr_type;

/* Written by the NimBLE host task (GAP events), read by ble_link_task
 * and the CLI. */
static volatile uint16_t conn_handle = BLE_HS_CONN_HANDLE_NONE;
static volatile bool frame_subscribed;
static volatile uint16_t mtu = BLE_ATT_MTU_DFLT;
static volatile bool streaming;
/* Negotiated link parameters, for the info JSON / CLI. */
static volatile uint16_t conn_itvl;  /* x1.25 ms */
static volatile uint8_t tx_phy = 1;  /* 1 = 1M, 2 = 2M */

/* ctrl writes land here (host task) and are executed by ble_link_task,
 * which owns the camera while sending. */
static QueueHandle_t cmd_queue;

static volatile struct {
	uint32_t connects;
	uint32_t frames;
	uint32_t bytes;
	uint32_t retries;   /* notify hit ENOMEM and was retried */
	uint32_t notifies;  /* successful notifications */
	uint32_t ctrl_full; /* retries while the controller had no free ACL buffer */
	uint32_t blocked_ms;/* time spent backing off */
	uint32_t aborted;   /* frame cut short (disconnect / unsubscribe) */
	uint32_t last_frame_ms;
} stats;

/* ---- GATT ---- */

static int info_json(char *buf, size_t len)
{
	int n = snprintf(buf, len,
		"{\"fw\":\"%s\",\"idf\":\"%s\",\"uptime_ms\":%" PRIu32 ","
		"\"heap\":%u,\"psram\":%u,\"mtu\":%u,\"itvl_us\":%u,\"phy\":%u,\"streaming\":%s,"
		"\"frames\":%" PRIu32 ",\"retries\":%" PRIu32 ",\"cam\":",
		FIRMWARE_VERSION, esp_get_idf_version(),
		(uint32_t)(esp_timer_get_time() / 1000),
		(unsigned)heap_caps_get_free_size(MALLOC_CAP_INTERNAL),
		(unsigned)heap_caps_get_free_size(MALLOC_CAP_SPIRAM),
		(unsigned)mtu, (unsigned)conn_itvl * 1250, (unsigned)tx_phy,
		streaming ? "true" : "false",
		stats.frames, stats.retries);
	if (n < 0 || (size_t)n >= len) return 0;
	n += camera_info_json(buf + n, len - n);
	if ((size_t)n + 8 > len) return 0;
	n += snprintf(buf + n, len - n, ",\"det\":");
	n += detector_info_json(buf + n, len - n);
	if ((size_t)n + 2 > len) return 0;
	buf[n++] = '}';
	buf[n] = '\0';
	return n;
}

static int gatt_access(uint16_t conn, uint16_t attr,
		       struct ble_gatt_access_ctxt *ctxt, void *arg)
{
	if (ctxt->op == BLE_GATT_ACCESS_OP_WRITE_CHR) {
		char cmd[CMD_MAX];
		uint16_t len = 0;
		if (ble_hs_mbuf_to_flat(ctxt->om, cmd, sizeof(cmd) - 1, &len) != 0) {
			return BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN;
		}
		cmd[len] = '\0';
		/* A full queue means the client is spamming; drop it. */
		xQueueSend(cmd_queue, cmd, 0);
		return 0;
	}
	if (ctxt->op == BLE_GATT_ACCESS_OP_READ_CHR && attr == info_handle) {
		/* NimBLE re-invokes this for each Read Blob and slices by
		 * offset itself, so long reads just work up to 512 bytes. */
		char buf[512];
		int n = info_json(buf, sizeof(buf));
		return os_mbuf_append(ctxt->om, buf, n) == 0 ? 0 : BLE_ATT_ERR_INSUFFICIENT_RES;
	}
	return BLE_ATT_ERR_UNLIKELY;
}

static const struct ble_gatt_svc_def gatt_svcs[] = {
	{
		.type = BLE_GATT_SVC_TYPE_PRIMARY,
		.uuid = &svc_uuid.u,
		.characteristics = (struct ble_gatt_chr_def[]) {
			{
				.uuid = &ctrl_uuid.u,
				.access_cb = gatt_access,
				.flags = BLE_GATT_CHR_F_WRITE,
			},
			{
				.uuid = &event_uuid.u,
				.access_cb = gatt_access,
				.flags = BLE_GATT_CHR_F_NOTIFY,
				.val_handle = &event_handle,
			},
			{
				.uuid = &info_uuid.u,
				.access_cb = gatt_access,
				.flags = BLE_GATT_CHR_F_READ,
				.val_handle = &info_handle,
			},
			{
				.uuid = &frame_uuid.u,
				.access_cb = gatt_access,
				.flags = BLE_GATT_CHR_F_NOTIFY,
				.val_handle = &frame_handle,
			},
			{ 0 },
		},
	},
	{ 0 },
};

/* ---- GAP ---- */

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

/*
 * Throughput knobs, all requests the central may refuse: 2M PHY,
 * 251-byte link-layer packets (data length extension) and a shorter
 * connection interval. macOS/iOS connect at 30 ms and only accept
 * updates that follow Apple's accessory rules (min >= 15 ms,
 * max >= min + 15 ms), hence 15..30 ms rather than 7.5 ms.
 * MTU is negotiated by the client; we offer 512 via
 * CONFIG_BT_NIMBLE_ATT_PREFERRED_MTU.
 */
static bool conn_param_retried;

static void request_interval(uint16_t conn)
{
	const struct ble_gap_upd_params p = {
		.itvl_min = 12,             /* x1.25 ms = 15 ms */
		.itvl_max = 24,             /* 30 ms */
		.latency = 0,
		.supervision_timeout = 400, /* x10 ms = 4 s */
	};
	int rc = ble_gap_update_params(conn, &p);
	if (rc) ESP_LOGW(TAG, "conn param request: %d", rc);
}

static void tune_link(uint16_t conn)
{
	conn_param_retried = false;
	int rc = ble_gap_set_prefered_le_phy(conn, BLE_GAP_LE_PHY_2M_MASK, BLE_GAP_LE_PHY_2M_MASK, 0);
	if (rc) ESP_LOGW(TAG, "2M PHY request: %d", rc);
	rc = ble_gap_set_data_len(conn, 251, 2120);
	if (rc) ESP_LOGW(TAG, "data length request: %d", rc);
	request_interval(conn);
}

static int gap_event(struct ble_gap_event *event, void *arg)
{
	switch (event->type) {
	case BLE_GAP_EVENT_CONNECT:
		if (event->connect.status == 0) {
			conn_handle = event->connect.conn_handle;
			stats.connects++;
			struct ble_gap_conn_desc d;
			if (ble_gap_conn_find(conn_handle, &d) == 0) conn_itvl = d.conn_itvl;
			ESP_LOGI(TAG, "client connected, interval %u x1.25 ms", conn_itvl);
			tune_link(conn_handle);
		} else {
			advertise();
		}
		break;

	case BLE_GAP_EVENT_DISCONNECT:
		ESP_LOGI(TAG, "client disconnected (reason 0x%x)", event->disconnect.reason);
		conn_handle = BLE_HS_CONN_HANDLE_NONE;
		frame_subscribed = false;
		streaming = false;
		mtu = BLE_ATT_MTU_DFLT;
		advertise();
		break;

	case BLE_GAP_EVENT_SUBSCRIBE:
		if (event->subscribe.attr_handle == frame_handle) {
			frame_subscribed = event->subscribe.cur_notify;
			if (!frame_subscribed) streaming = false;
			ESP_LOGI(TAG, "frame notifications %s", frame_subscribed ? "on" : "off");
		}
		break;

	case BLE_GAP_EVENT_CONN_UPDATE: {
		if (event->conn_update.status) {
			ESP_LOGW(TAG, "conn update failed: 0x%x", event->conn_update.status);
			/* Typically 0x2a, a collision with the PHY update we
			 * asked for at the same time -- ask once more. */
			if (!conn_param_retried) {
				conn_param_retried = true;
				request_interval(event->conn_update.conn_handle);
			}
		}
		struct ble_gap_conn_desc d;
		if (ble_gap_conn_find(event->conn_update.conn_handle, &d) == 0) {
			conn_itvl = d.conn_itvl;
			ESP_LOGI(TAG, "conn interval %u.%02u ms, latency %u",
				 d.conn_itvl * 125 / 100, d.conn_itvl * 125 % 100, d.conn_latency);
		}
		break;
	}

	case BLE_GAP_EVENT_PHY_UPDATE_COMPLETE:
		tx_phy = event->phy_updated.tx_phy;
		ESP_LOGI(TAG, "phy tx %uM rx %uM", event->phy_updated.tx_phy, event->phy_updated.rx_phy);
		break;

	case BLE_GAP_EVENT_DATA_LEN_CHG:
		ESP_LOGI(TAG, "data length tx %u rx %u",
			 event->data_len_chg.max_tx_octets, event->data_len_chg.max_rx_octets);
		break;

	case BLE_GAP_EVENT_MTU:
		mtu = event->mtu.value;
		ESP_LOGI(TAG, "mtu %u", (unsigned)mtu);
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
	cmd_queue = xQueueCreate(4, CMD_MAX);

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

/* ---- sending ---- */

static bool link_up(void)
{
	return conn_handle != BLE_HS_CONN_HANDLE_NONE;
}

/*
 * One notification, retried while NimBLE is out of buffers -- a frame
 * is useless with a hole in it, so unlike dcc's sniffer stream we never
 * drop a chunk. Gives up only if the link goes away. Buffers free up
 * as the controller gets packets out (no host event for that), so just
 * back off briefly; the link, not this loop, is the bottleneck (~10-15
 * KB/s against macOS, which holds the interval at 30 ms).
 * ble_gatts_notify_custom() frees the mbuf on failure too, so it's
 * rebuilt on each attempt.
 */
static bool notify(uint16_t handle, const void *data, size_t len)
{
	while (link_up()) {
		struct os_mbuf *om = ble_hs_mbuf_from_flat(data, len);
		int rc = om ? ble_gatts_notify_custom(conn_handle, handle, om) : BLE_HS_ENOMEM;
		if (rc == 0) {
			stats.notifies++;
			return true;
		}
		if (rc != BLE_HS_ENOMEM && rc != BLE_HS_EBUSY) {
			ESP_LOGW(TAG, "notify failed: %d", rc);
			return false;
		}
		stats.retries++;
		if (ble_hs_hci_avail_pkts == 0) stats.ctrl_full++;
		vTaskDelay(pdMS_TO_TICKS(2));
		stats.blocked_ms += 2;
	}
	return false;
}

static void reply(const char *fmt, ...)
{
	char buf[128];
	va_list ap;
	va_start(ap, fmt);
	int n = vsnprintf(buf, sizeof(buf), fmt, ap);
	va_end(ap);
	if (n < 0) return;
	size_t cap = mtu - 3;
	notify(event_handle, buf, (size_t)n < cap ? (size_t)n : cap);
}

static void put_u16(uint8_t *p, uint16_t v) { p[0] = v; p[1] = v >> 8; }
static void put_u32(uint8_t *p, uint32_t v) { p[0] = v; p[1] = v >> 8; p[2] = v >> 16; p[3] = v >> 24; }

static uint8_t chunk[512];

/* One header record + data chunks for any payload (camera frame or
 * detector snapshot). Returns true if every chunk went out. */
static bool send_payload(const uint8_t *buf, size_t len, uint16_t w, uint16_t h_px,
			 uint8_t format, uint32_t ts_ms)
{
	static uint16_t frame_id;
	frame_id++;

	/* esp_rom_crc32_le(0, ...) is the standard zlib/IEEE CRC-32 (the
	 * ROM routine does the pre/post inversion itself). */
	uint8_t *h = chunk;
	h[0] = REC_HEADER;
	put_u16(h + 1, frame_id);
	put_u32(h + 3, len);
	put_u16(h + 7, w);
	put_u16(h + 9, h_px);
	h[11] = format;
	put_u32(h + 12, ts_ms);
	put_u32(h + 16, esp_rom_crc32_le(0, buf, len));
	put_u16(h + 20, 0);  /* reserved */

	bool ok = notify(frame_handle, h, HEADER_LEN);
	size_t off = 0;
	while (ok && off < len && frame_subscribed) {
		size_t cap = mtu - 3;
		if (cap > sizeof(chunk)) cap = sizeof(chunk);
		size_t n = len - off;
		if (n > cap - DATA_HDR) n = cap - DATA_HDR;

		chunk[0] = REC_DATA;
		put_u16(chunk + 1, frame_id);
		put_u32(chunk + 3, off);
		memcpy(chunk + DATA_HDR, buf + off, n);
		ok = notify(frame_handle, chunk, DATA_HDR + n);
		off += n;
	}
	return ok && off == len;
}

static void send_frame(void)
{
	int64_t t0 = esp_timer_get_time();
	camera_fb_t *fb = camera_grab();
	if (!fb) {
		reply("err capture failed");
		streaming = false;
		return;
	}
	size_t len = fb->len;
	bool ok = send_payload(fb->buf, len, fb->width, fb->height, fb->format,
			       (uint32_t)(fb->timestamp.tv_sec * 1000 + fb->timestamp.tv_usec / 1000));
	camera_release(fb);

	if (ok) {
		stats.frames++;
		stats.bytes += len;
		stats.last_frame_ms = (esp_timer_get_time() - t0) / 1000;
	} else {
		stats.aborted++;
	}
}

/* The detector's last inference: model input + all anchor outputs. */
static void send_detection(void)
{
	size_t n = detector_snapshot_size();
	if (!n) {
		reply("err no inference yet");
		return;
	}
	uint8_t *buf = heap_caps_malloc(n, MALLOC_CAP_SPIRAM);
	if (!buf) {
		reply("err out of memory");
		return;
	}
	n = detector_snapshot(buf, n);
	uint16_t res = buf[4] | buf[5] << 8;
	reply("ok detsnap %u", (unsigned)n);
	if (!send_payload(buf, n, res, res, DET_SNAPSHOT_FORMAT, (uint32_t)(esp_timer_get_time() / 1000)))
		stats.aborted++;
	heap_caps_free(buf);
}

static void run_command(char *cmd)
{
	char *argv[4] = { 0 };
	int argc = 0;
	for (char *tok = strtok(cmd, " \t\r\n"); tok && argc < 4; tok = strtok(NULL, " \t\r\n")) {
		argv[argc++] = tok;
	}
	if (argc == 0) return;
	const char *c = argv[0], *a = argv[1];

	if (strcmp(c, "info") == 0) {
		reply("ok info");
	} else if (strcmp(c, "snap") == 0) {
		if (!frame_subscribed) { reply("err subscribe to frame first"); return; }
		reply("ok snap");
		send_frame();
	} else if (strcmp(c, "detsnap") == 0) {
		if (!frame_subscribed) { reply("err subscribe to frame first"); return; }
		send_detection();
	} else if (strcmp(c, "stream") == 0) {
		if (!frame_subscribed) { reply("err subscribe to frame first"); return; }
		streaming = a && strcmp(a, "on") == 0;
		reply("ok stream %s", streaming ? "on" : "off");
	} else if (strcmp(c, "fmt") == 0) {
		int f = a ? camera_format_from_name(a) : -1;
		if (f < 0) { reply("err fmt jpeg|gray|rgb565"); return; }
		esp_err_t err = camera_set_format(f);
		reply(err == ESP_OK ? "ok fmt %s" : "err fmt %s failed", a);
	} else if (strcmp(c, "res") == 0) {
		int fs = a ? camera_framesize_from_name(a) : -1;
		if (fs < 0) { reply("err res unknown size"); return; }
		esp_err_t err = camera_set_framesize(fs);
		reply(err == ESP_OK ? "ok res %s" : "err res %s failed", a);
	} else if (strcmp(c, "q") == 0) {
		esp_err_t err = a ? camera_set_quality(atoi(a)) : ESP_ERR_INVALID_ARG;
		reply(err == ESP_OK ? "ok q %s" : "err q 4..63", a ? a : "");
	} else if (strcmp(c, "set") == 0) {
		if (argc < 3) { reply("err set <param> <value>"); return; }
		esp_err_t err = camera_set_param(a, atoi(argv[2]));
		reply(err == ESP_OK ? "ok set %s %s" : "err set %s %s: %s", a, argv[2], esp_err_to_name(err));
	} else if (strcmp(c, "reg") == 0) {
		if (!a) { reply("err reg <0xBAA> [value]"); return; }
		int reg = strtol(a, NULL, 0);
		if (argc > 2 && camera_reg_set(reg, strtol(argv[2], NULL, 0)) != ESP_OK) {
			reply("err reg write failed"); return;
		}
		int v = camera_reg_get(reg);
		if (v < 0) reply("err reg read failed");
		else reply("ok reg 0x%03x 0x%02x", reg, v);
	} else if (strcmp(c, "xclk") == 0) {
		esp_err_t err = a ? camera_set_xclk(atoi(a)) : ESP_ERR_INVALID_ARG;
		reply(err == ESP_OK ? "ok xclk %s" : "err xclk 5..24", a ? a : "");
	} else {
		reply("err unknown command '%s'", c);
	}
}

void ble_link_task(void *arg)
{
	char cmd[CMD_MAX];
	uint32_t det_seq = detector_last(NULL, NULL);
	while (1) {
		/* Push every new inference result as an event, so the page
		 * doesn't have to poll: "det <seq> <score> hand|none". */
		float score;
		bool hand;
		uint32_t seq = detector_last(&score, &hand);
		if (seq != det_seq && link_up()) {
			reply("det %lu %.3f %s", (unsigned long)seq, score, hand ? "hand" : "none");
		}
		det_seq = seq;

		/* While streaming, poll commands between frames; otherwise
		 * block on the queue. */
		TickType_t wait = streaming ? 0 : pdMS_TO_TICKS(500);
		if (xQueueReceive(cmd_queue, cmd, wait) == pdTRUE) {
			run_command(cmd);
		}
		if (streaming && frame_subscribed) {
			send_frame();
		}
	}
}

/* ---- CLI ---- */

static int cmd_ble(int argc, char **argv)
{
	bool connected = link_up();

	printf("name           %s\n", BLE_DEVICE_NAME);
	printf("state          %s\n",
	       !connected ? "advertising" : streaming ? "streaming"
	       : frame_subscribed ? "connected, subscribed" : "connected");
	printf("mtu            %u\n", (unsigned)mtu);
	printf("interval       %u us, PHY %uM\n", (unsigned)conn_itvl * 1250, (unsigned)tx_phy);
	printf("connects       %" PRIu32 "\n", stats.connects);
	printf("frames sent    %" PRIu32 " (%" PRIu32 " bytes)\n", stats.frames, stats.bytes);
	printf("last frame     %" PRIu32 " ms\n", stats.last_frame_ms);
	printf("frames aborted %" PRIu32 "\n", stats.aborted);
	printf("notifications  %" PRIu32 "\n", stats.notifies);
	printf("notify retries %" PRIu32 " (controller full on %" PRIu32 ")\n", stats.retries, stats.ctrl_full);
	printf("backed off     %" PRIu32 " ms\n", stats.blocked_ms);
	printf("ctrl ACL free  %u\n", (unsigned)ble_hs_hci_avail_pkts);
	printf("msys free      %d (both pools)\n", os_msys_num_free());
	return 0;
}

void ble_link_register_cli_commands(void)
{
	const esp_console_cmd_t ble_cmd = {
		.command = "ble",
		.help = "BLE link state and counters (web diagnostic page)",
		.func = &cmd_ble,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&ble_cmd));
}
