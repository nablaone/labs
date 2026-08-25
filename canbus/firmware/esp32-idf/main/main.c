#include <stdio.h>
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
