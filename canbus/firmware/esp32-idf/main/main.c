#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <stdint.h>
#include <inttypes.h>
#include <unistd.h>
#include <fcntl.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/semphr.h"
#include "driver/gpio.h"
#include "driver/uart.h"
#include "driver/uart_vfs.h"
#include "driver/twai.h"
#include "esp_log.h"
#include "esp_console.h"
#include "esp_idf_version.h"
#include "argtable3/argtable3.h"
#include "linenoise/linenoise.h"

/*
 * LED on GPIO2 (onboard LED) and button on GPIO0 (onboard BOOT button) --
 * both onboard, no breadboard wiring needed. Both are strapping pins
 * (sampled at boot to select flash/boot mode), which is why the earlier
 * Zephyr app avoided them in favor of an external button on GPIO33 -- but
 * once the app is running, GPIO0 reads like any other input (it's only
 * sampled at reset), and the onboard LED's light loading on GPIO2 doesn't
 * disturb boot-mode sensing in practice (confirmed on real hardware). The
 * board already has an external pull-up on GPIO0 for its BOOT button;
 * gpio_pullup_en() below just reinforces it.
 */
#define LED_GPIO    GPIO_NUM_2
#define BUTTON_GPIO GPIO_NUM_0

/*
 * TWAI (CAN) TX/RX to the SN65HVD230 transceiver -- see
 * ../../../docs/can-bus-bringup-plan.md for the full pin-choice reasoning
 * (not strapping, not input-only, not UART0/flash-SPI, free). Silkscreen
 * on this board's DevKit V1-style clone prints these as D21/D22 -- same
 * physical pins as GPIO21/22, just an alias.
 */
#define CAN_TX_GPIO GPIO_NUM_21
#define CAN_RX_GPIO GPIO_NUM_22

/* Scratch ID for the self-test loopback frame -- deliberately in the gap
 * docs/can-message-spec.md leaves unallocated (0x100-0x6FF), not a real
 * message. */
#define CAN_SELFTEST_ID 0x100

#define POLL_MS       100
#define HEARTBEAT_MS  1000
#define DISPLAY_MS    10000

#define FIRMWARE_VERSION "0.2.0"
#define CLI_PROMPT "canbus> "

static const char *TAG = "canbus";

/*
 * Shared state -- a stand-in for later CAN-driven events (each interesting
 * bus event will eventually bump the counter). Touched concurrently by
 * tasks that may run on either of the ESP32's two cores, so it's guarded
 * by a mutex rather than relying on plain reads/writes being atomic.
 */
static struct {
	uint32_t counter;
	uint32_t heartbeat_period_ms;
} state = {
	.counter = 0,
	.heartbeat_period_ms = HEARTBEAT_MS,
};
static SemaphoreHandle_t state_mutex;

static uint32_t counter_read(void)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	uint32_t value = state.counter;
	xSemaphoreGive(state_mutex);
	return value;
}

static void counter_increment(void)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	state.counter++;
	xSemaphoreGive(state_mutex);
}

static uint32_t heartbeat_period_read(void)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	uint32_t value = state.heartbeat_period_ms;
	xSemaphoreGive(state_mutex);
	return value;
}

static void heartbeat_period_set(uint32_t ms)
{
	xSemaphoreTake(state_mutex, portMAX_DELAY);
	state.heartbeat_period_ms = ms;
	xSemaphoreGive(state_mutex);
}

/* Polls the counter every 100ms; toggles the LED whenever it has changed
 * since the last poll. */
static void led_task(void *arg)
{
	uint32_t last_seen = counter_read();
	bool led_on = false;

	while (1) {
		uint32_t current = counter_read();

		if (current != last_seen) {
			led_on = !led_on;
			gpio_set_level(LED_GPIO, led_on);
			ESP_LOGI(TAG, "led %s (counter=%" PRIu32 ")", led_on ? "on" : "off", current);
			last_seen = current;
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}

/* Increments the counter on its own, at a period settable at runtime via
 * the "rate" CLI command -- re-read each cycle rather than latched, so a
 * rate change takes effect on the next tick without needing to interrupt
 * an in-progress wait. */
static void heartbeat_task(void *arg)
{
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(heartbeat_period_read()));
		counter_increment();
	}
}

/* Polls the button every 100ms; increments the counter on every poll where
 * it reads pressed, so holding it down keeps incrementing rather than
 * counting only the initial press. */
static void button_task(void *arg)
{
	while (1) {
		/* active-low: button pressed means the pin reads 0 */
		if (gpio_get_level(BUTTON_GPIO) == 0) {
			counter_increment();
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}

/*
 * Stage A/B bring-up (see docs/can-bus-bringup-plan.md). The driver runs
 * in TWAI_MODE_NORMAL day to day (real bus traffic, real ACKs) but the
 * self-test ("can loop"/"can xcvr") needs TWAI_MODE_NO_ACK -- a lone
 * node's transmitted frame would otherwise be treated as a "no ACK"
 * error and retried forever, since there's no peer to ACK it. can_reinit()
 * does a full stop/uninstall/reinstall to switch between the two; the
 * self-test commands use it to drop into NO_ACK mode temporarily and
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
		 "can status: state=%d msgs_to_tx=%" PRIu32 " msgs_to_rx=%" PRIu32
		 " tx_err=%" PRIu32 " rx_err=%" PRIu32 " tx_failed=%" PRIu32
		 " arb_lost=%" PRIu32 " bus_err=%" PRIu32,
		 status.state, status.msgs_to_tx, status.msgs_to_rx,
		 status.tx_error_counter, status.rx_error_counter,
		 status.tx_failed_count, status.arb_lost_count, status.bus_error_count);
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
		ESP_LOGE(TAG, "can selftest: transmit failed: %s", esp_err_to_name(err));
		can_log_status();
		return false;
	}

	twai_message_t rx_msg;
	err = twai_receive(&rx_msg, pdMS_TO_TICKS(200));
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "can selftest: no frame looped back: %s", esp_err_to_name(err));
		can_log_status();
		return false;
	}

	bool ok = rx_msg.identifier == tx_msg.identifier &&
		  rx_msg.data_length_code == tx_msg.data_length_code &&
		  rx_msg.data[0] == tx_msg.data[0];

	if (!ok) {
		ESP_LOGE(TAG, "can selftest: loopback mismatch (id=0x%" PRIx32 " dlc=%d data=0x%02x)",
			  rx_msg.identifier, rx_msg.data_length_code, rx_msg.data[0]);
	}

	return ok;
}

/* Shared by "can loop" and "can xcvr" -- electrically identical test
 * (NO_ACK self-reception loopback), the only difference is what's
 * physically wired between D21/D22 when you run it. */
static bool can_run_selftest(void)
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

#define CAN_SNIFF_TIMEOUT_MS (30 * 1000)

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

	twai_message_t msg = {
		.identifier = strtoul(id_str, NULL, 16),
		.data_length_code = data_hex_len / 2,
	};
	for (int i = 0; i < msg.data_length_code; i++) {
		char byte_str[3] = { data_str[i * 2], data_str[i * 2 + 1], '\0' };
		msg.data[i] = (uint8_t)strtoul(byte_str, NULL, 16);
	}

	esp_err_t err = twai_transmit(&msg, pdMS_TO_TICKS(1000));
	if (err != ESP_OK) {
		printf("send failed: %s\n", esp_err_to_name(err));
		can_log_status();
		return 1;
	}

	printf("sent %03" PRIX32 "#", msg.identifier);
	for (int i = 0; i < msg.data_length_code; i++) {
		printf("%02X", msg.data[i]);
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

static int cmd_version(int argc, char **argv)
{
	printf("canbus-esp32-idf %s (ESP-IDF %s)\n", FIRMWARE_VERSION, esp_get_idf_version());
	return 0;
}

static int cmd_counter(int argc, char **argv)
{
	printf("%" PRIu32 "\n", counter_read());
	return 0;
}

static struct {
	struct arg_int *n;
	struct arg_end *end;
} rate_args;

static int cmd_rate(int argc, char **argv)
{
	int nerrors = arg_parse(argc, argv, (void **)&rate_args);
	if (nerrors) {
		arg_print_errors(stderr, rate_args.end, argv[0]);
		return 1;
	}

	int n = rate_args.n->ival[0];
	if (n <= 0) {
		printf("N must be a positive integer (period = N * 100ms)\n");
		return 1;
	}

	heartbeat_period_set((uint32_t)n * 100);
	printf("heartbeat period set to %dms\n", n * 100);
	return 0;
}

static volatile bool cli_exit_requested;

static int cmd_exit(int argc, char **argv)
{
	cli_exit_requested = true;
	return 0;
}

static void register_cli_commands(void)
{
	esp_console_register_help_command();

	const esp_console_cmd_t version_cmd = {
		.command = "version",
		.help = "Show firmware version",
		.func = &cmd_version,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&version_cmd));

	const esp_console_cmd_t counter_cmd = {
		.command = "counter",
		.help = "Show the current excitement counter value",
		.func = &cmd_counter,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&counter_cmd));

	rate_args.n = arg_int1(NULL, NULL, "<N>", "heartbeat period, in units of 100ms");
	rate_args.end = arg_end(1);
	const esp_console_cmd_t rate_cmd = {
		.command = "rate",
		.help = "Set the heartbeat period to N*100ms",
		.func = &cmd_rate,
		.argtable = &rate_args,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&rate_cmd));

	const esp_console_cmd_t exit_cmd = {
		.command = "exit",
		.help = "Leave CLI mode, resume logging",
		.func = &cmd_exit,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&exit_cmd));

	const esp_console_cmd_t can_cmd = {
		.command = "can",
		.help = "CAN: loop|xcvr (self-test) | sniff (30s) | send <id_hex>#<data_hex>",
		.func = &cmd_can,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&can_cmd));
}

/*
 * Installs the interrupt-driven UART driver and points the VFS console
 * layer at it -- needed for linenoise's line editing (backspace, history)
 * to work; without it stdin falls back to a busy-polling boot-console
 * reader with no editing support.
 */
static void console_init(void)
{
	fflush(stdout);
	fsync(fileno(stdout));

	uart_vfs_dev_port_set_rx_line_endings(CONFIG_ESP_CONSOLE_UART_NUM, ESP_LINE_ENDINGS_CR);
	uart_vfs_dev_port_set_tx_line_endings(CONFIG_ESP_CONSOLE_UART_NUM, ESP_LINE_ENDINGS_CRLF);

	const uart_config_t uart_config = {
		.baud_rate = CONFIG_ESP_CONSOLE_UART_BAUDRATE,
		.data_bits = UART_DATA_8_BITS,
		.parity = UART_PARITY_DISABLE,
		.stop_bits = UART_STOP_BITS_1,
		.source_clk = UART_SCLK_DEFAULT,
	};
	ESP_ERROR_CHECK(uart_driver_install(CONFIG_ESP_CONSOLE_UART_NUM, 256, 0, 0, NULL, 0));
	ESP_ERROR_CHECK(uart_param_config(CONFIG_ESP_CONSOLE_UART_NUM, &uart_config));
	uart_vfs_dev_use_driver(CONFIG_ESP_CONSOLE_UART_NUM);

	/* The driver-installed VFS UART defaults stdin/stdout to non-blocking;
	 * ESP-IDF's own esp_console_new_repl_uart() forces blocking mode too. */
	fcntl(fileno(stdin), F_SETFL, 0);
	fcntl(fileno(stdout), F_SETFL, 0);

	const esp_console_config_t console_config = {
		.max_cmdline_args = 8,
		.max_cmdline_length = 256,
	};
	ESP_ERROR_CHECK(esp_console_init(&console_config));

	/* Without this, newlib's stdio buffering means fgetc()'s first call
	 * issues one large underlying read() rather than a 1-byte one; the
	 * UART VFS driver's read() blocks *inside that single call* character
	 * by character until it either fills the requested size or sees '\n'
	 * -- so nothing is visible while typing and the whole line (echo +
	 * response) appears at once on Enter. ESP-IDF's own REPL task
	 * (esp_console_repl_task(), which we don't use -- we run our own
	 * console_task/run_cli() instead) sets this for exactly this reason. */
	setvbuf(stdin, NULL, _IONBF, 0);

	linenoiseHistorySetMaxLen(20);

	/* Forced rather than probed: linenoise's normal mode redraws the
	 * line by cursor-position escape sequences, which didn't render in
	 * minicom (garbled/invisible input while typing) even though minicom
	 * answers the terminal capability probe as if it supports them. Dumb
	 * mode just echoes each character as it's typed -- no cursor
	 * repositioning -- which is all this CLI needs and is confirmed
	 * working. */
	linenoiseSetDumbMode(1);

	register_cli_commands();
}

/*
 * Runs the read-eval-print loop until "exit". Logging is silenced for the
 * duration so the prompt/output isn't interleaved with task log lines, and
 * restored on the way out.
 */
static void run_cli(void)
{
	cli_exit_requested = false;
	esp_log_level_set("*", ESP_LOG_NONE);
	printf("\n-- CLI mode -- 'help' for commands, 'exit' to leave --\n");

	while (!cli_exit_requested) {
		char *line = linenoise(CLI_PROMPT);
		if (line == NULL) {
			/* linenoise error/EOF -- not a normal way to leave,
			 * but bail rather than spin. */
			break;
		}

		if (line[0] != '\0') {
			linenoiseHistoryAdd(line);

			int ret;
			esp_err_t err = esp_console_run(line, &ret);
			if (err == ESP_ERR_NOT_FOUND) {
				printf("Unknown command: '%s'\n", line);
			} else if (err != ESP_OK && err != ESP_ERR_INVALID_ARG) {
				printf("Command error: 0x%x\n", err);
			}
		}

		linenoiseFree(line);
	}

	printf("-- leaving CLI mode --\n");
	esp_log_level_set("*", ESP_LOG_INFO);

	/* Discards any stray buffered byte (e.g. a trailing '\n' from a
	 * terminal that sends CRLF for Enter -- the UART's RX line-ending
	 * mode only special-cases '\r') so it doesn't immediately re-trigger
	 * console_task's "enter CLI mode" check below. */
	uart_flush_input(CONFIG_ESP_CONSOLE_UART_NUM);
}

/* Waits for Enter while logs are flowing normally; switches into CLI mode
 * once it sees one, then goes back to waiting once run_cli() returns. */
static void console_task(void *arg)
{
	while (1) {
		int c = fgetc(stdin);
		if (c == '\n' || c == '\r') {
			run_cli();
		}
	}
}

void app_main(void)
{
	gpio_reset_pin(LED_GPIO);
	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);

	gpio_reset_pin(BUTTON_GPIO);
	gpio_set_direction(BUTTON_GPIO, GPIO_MODE_INPUT);
	gpio_pullup_en(BUTTON_GPIO);
	gpio_pulldown_dis(BUTTON_GPIO);

	state_mutex = xSemaphoreCreateMutex();
	console_init();

	ESP_LOGI(TAG, "CAN self-test: %s", can_run_selftest() ? "PASS" : "FAIL");

	xTaskCreate(led_task, "led", 3072, NULL, 5, NULL);
	xTaskCreate(heartbeat_task, "heartbeat", 3072, NULL, 5, NULL);
	xTaskCreate(button_task, "button", 3072, NULL, 5, NULL);
	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);

	/* Main task: display the counter every 10s (suppressed along with
	 * everything else while CLI mode has the log level muted). */
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(DISPLAY_MS));
		ESP_LOGI(TAG, "excitement counter = %" PRIu32, counter_read());
	}
}
