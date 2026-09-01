#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/twai.h"
#include "esp_log.h"
#include "esp_console.h"

#include "node_config.h"
#include "can.h"

static const char *TAG = "can";

/* Scratch ID for the self-test loopback frame -- deliberately in the gap
 * ../../../docs/can-message-spec.md leaves unallocated (0x100-0x6FF), not
 * a real message. */
#define CAN_SELFTEST_ID 0x100
#define CAN_SNIFF_TIMEOUT_MS (30 * 1000)

/*
 * Stage A/B bring-up (see ../../../docs/can-bus-bringup-plan.md). The
 * driver runs in TWAI_MODE_NORMAL day to day (real bus traffic, real
 * ACKs) but the self-test ("can loop"/"can xcvr") needs TWAI_MODE_NO_ACK
 * -- a lone node's transmitted frame would otherwise be treated as a "no
 * ACK" error and retried forever, since there's no peer to ACK it.
 * can_reinit() does a full stop/uninstall/reinstall to switch between the
 * two (harmless to call before any driver has ever been installed --
 * twai_stop()/twai_driver_uninstall()'s errors in that case are ignored);
 * the self-test commands use it to drop into NO_ACK mode temporarily and
 * always leave the driver back in NORMAL mode on the way out.
 */
static void can_reinit(twai_mode_t mode)
{
	twai_stop();
	twai_driver_uninstall();

	twai_general_config_t g_config =
		TWAI_GENERAL_CONFIG_DEFAULT(CAN_TX_GPIO, CAN_RX_GPIO, mode);
	twai_timing_config_t t_config = TWAI_TIMING_CONFIG_500KBITS();
	twai_filter_config_t f_config = TWAI_FILTER_CONFIG_ACCEPT_ALL();

	ESP_ERROR_CHECK(twai_driver_install(&g_config, &t_config, &f_config));
	ESP_ERROR_CHECK(twai_start());
}

/* Logs TWAI's error/state counters -- distinguishes "frame never actually
 * left the controller" (msgs_to_tx still nonzero) from "it tried and hit
 * bus errors" (bus_error_count/tx_error_counter climbing, pointing at
 * wiring: TX/RX swapped, no transceiver power, missing termination) from
 * "controller itself never got configured right" (state != RUNNING). */
static void can_log_status(void)
{
	twai_status_info_t status;
	if (twai_get_status_info(&status) != ESP_OK) {
		return;
	}

	ESP_LOGI(TAG,
		 "status: state=%d msgs_to_tx=%" PRIu32 " msgs_to_rx=%" PRIu32
		 " tx_err=%" PRIu32 " rx_err=%" PRIu32 " tx_failed=%" PRIu32
		 " arb_lost=%" PRIu32 " bus_err=%" PRIu32,
		 status.state, status.msgs_to_tx, status.msgs_to_rx,
		 status.tx_error_counter, status.rx_error_counter,
		 status.tx_failed_count, status.arb_lost_count, status.bus_error_count);
}

bool can_send(uint32_t id, const uint8_t *data, size_t len)
{
	if (len > 8) {
		ESP_LOGE(TAG, "send: %u bytes is too long (max 8)", (unsigned)len);
		return false;
	}

	twai_message_t msg = {
		.identifier = id,
		.data_length_code = len,
	};
	memcpy(msg.data, data, len);

	esp_err_t err = twai_transmit(&msg, pdMS_TO_TICKS(1000));
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "send failed: %s", esp_err_to_name(err));
		can_log_status();
		return false;
	}

	return true;
}

bool can_send_u32(uint32_t id, uint32_t value)
{
	uint8_t data[4] = {
		(uint8_t)(value & 0xFF),
		(uint8_t)((value >> 8) & 0xFF),
		(uint8_t)((value >> 16) & 0xFF),
		(uint8_t)((value >> 24) & 0xFF),
	};
	return can_send(id, data, sizeof(data));
}

/* Transmits one frame and expects it back via the transceiver's own
 * electrical loopback -- pass confirms TX pin, RX pin, and the
 * transceiver chip are all wired and working. Caller must already have
 * the driver in TWAI_MODE_NO_ACK (see can_reinit()). */
static bool can_selftest(void)
{
	twai_message_t tx_msg = {
		.self = 1, /* self reception request -- without this, NO_ACK mode
			    * transmits fine but never queues the frame for our
			    * own twai_receive() to see, no matter how good the
			    * physical wiring is. See ESP-IDF's own
			    * examples/peripherals/twai/twai_self_test. */
		.identifier = CAN_SELFTEST_ID,
		.data_length_code = 1,
		.data = { 0xA5 },
	};

	esp_err_t err = twai_transmit(&tx_msg, pdMS_TO_TICKS(100));
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "selftest: transmit failed: %s", esp_err_to_name(err));
		can_log_status();
		return false;
	}

	twai_message_t rx_msg;
	err = twai_receive(&rx_msg, pdMS_TO_TICKS(200));
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "selftest: no frame looped back: %s", esp_err_to_name(err));
		can_log_status();
		return false;
	}

	bool ok = rx_msg.identifier == tx_msg.identifier &&
		  rx_msg.data_length_code == tx_msg.data_length_code &&
		  rx_msg.data[0] == tx_msg.data[0];

	if (!ok) {
		ESP_LOGE(TAG, "selftest: loopback mismatch (id=0x%" PRIx32 " dlc=%d data=0x%02x)",
			  rx_msg.identifier, rx_msg.data_length_code, rx_msg.data[0]);
	}

	return ok;
}

bool can_run_selftest(void)
{
	can_reinit(TWAI_MODE_NO_ACK);
	bool ok = can_selftest();
	can_reinit(TWAI_MODE_NORMAL);
	return ok;
}

static int cmd_can_loop(void)
{
	printf("Testing GPIO loopback -- wire D21 directly to D22 with a plain "
	       "jumper, transceiver disconnected...\n");
	bool ok = can_run_selftest();
	printf(ok ? "Loopback test: PASS\n"
		   : "Loopback test: FAIL -- see docs/can-bus-bringup-plan.md\n");
	return ok ? 0 : 1;
}

static int cmd_can_xcvr(void)
{
	printf("Testing transceiver -- wire the SN65HVD230 normally on D21/D22 "
	       "(CTX->D21, CRX->D22, VCC/GND powered)...\n");
	bool ok = can_run_selftest();
	printf(ok ? "Transceiver test: PASS\n"
		   : "Transceiver test: FAIL -- see docs/can-bus-bringup-plan.md\n");
	return ok ? 0 : 1;
}

/* Prints received frames as "<id_hex>#<data_hex>" (matching cantool.py's
 * output and cansend's input format) for a fixed CAN_SNIFF_TIMEOUT_MS,
 * then returns -- no longer waits for Enter (that relied on polling the
 * console UART for a stray keypress mid-loop, which added a variable
 * this command's own correctness didn't need to depend on; a fixed
 * timeout is simpler and behaves identically whether a human or a
 * script is driving the CLI). */
static int cmd_can_sniff(void)
{
	printf("Sniffing for %ds...\n", CAN_SNIFF_TIMEOUT_MS / 1000);

	TickType_t end = xTaskGetTickCount() + pdMS_TO_TICKS(CAN_SNIFF_TIMEOUT_MS);
	while (xTaskGetTickCount() < end) {
		twai_message_t msg;
		if (twai_receive(&msg, pdMS_TO_TICKS(200)) == ESP_OK) {
			printf("%03" PRIX32 "#", msg.identifier);
			for (int i = 0; i < msg.data_length_code; i++) {
				printf("%02X", msg.data[i]);
			}
			printf("\n");
		}
	}

	printf("Done.\n");
	return 0;
}

/* Parses "<id_hex>#<data_hex>" (e.g. "123#DEADBEEF") and transmits it --
 * same frame syntax as cantool.py's "send" and can-utils' cansend. */
static int cmd_can_send(const char *frame)
{
	const char *hash = strchr(frame, '#');
	if (!hash) {
		printf("bad frame '%s', expected <id_hex>#<data_hex>, e.g. 123#DEADBEEF\n", frame);
		return 1;
	}

	size_t id_len = (size_t)(hash - frame);
	if (id_len == 0 || id_len >= 9) {
		printf("bad id in '%s'\n", frame);
		return 1;
	}
	char id_str[9];
	memcpy(id_str, frame, id_len);
	id_str[id_len] = '\0';

	const char *data_str = hash + 1;
	size_t data_hex_len = strlen(data_str);
	if (data_hex_len % 2 != 0 || data_hex_len > 16) {
		printf("bad data in '%s' (need an even number of hex chars, up to 16 = 8 bytes)\n", frame);
		return 1;
	}

	uint8_t data[8];
	size_t len = data_hex_len / 2;
	for (size_t i = 0; i < len; i++) {
		char byte_str[3] = { data_str[i * 2], data_str[i * 2 + 1], '\0' };
		data[i] = (uint8_t)strtoul(byte_str, NULL, 16);
	}

	uint32_t id = strtoul(id_str, NULL, 16);
	if (!can_send(id, data, len)) {
		printf("send failed\n");
		return 1;
	}

	printf("sent %03" PRIX32 "#", id);
	for (size_t i = 0; i < len; i++) {
		printf("%02X", data[i]);
	}
	printf("\n");
	return 0;
}

static int cmd_can(int argc, char **argv)
{
	if (argc < 2) {
		printf("usage: can <loop|xcvr|sniff|send <id_hex>#<data_hex>>\n");
		return 1;
	}

	if (strcmp(argv[1], "loop") == 0) {
		return cmd_can_loop();
	} else if (strcmp(argv[1], "xcvr") == 0) {
		return cmd_can_xcvr();
	} else if (strcmp(argv[1], "sniff") == 0) {
		return cmd_can_sniff();
	} else if (strcmp(argv[1], "send") == 0) {
		if (argc < 3) {
			printf("usage: can send <id_hex>#<data_hex>\n");
			return 1;
		}
		return cmd_can_send(argv[2]);
	}

	printf("unknown can subcommand: '%s'\n", argv[1]);
	return 1;
}

void can_register_cli_commands(void)
{
	const esp_console_cmd_t can_cmd = {
		.command = "can",
		.help = "CAN: loop|xcvr (self-test) | sniff (30s) | send <id_hex>#<data_hex>",
		.func = &cmd_can,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&can_cmd));
}
