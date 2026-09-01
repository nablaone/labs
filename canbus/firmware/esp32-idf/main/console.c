#include <stdio.h>
#include <stdbool.h>
#include <unistd.h>
#include <fcntl.h>

#include "driver/uart.h"
#include "driver/uart_vfs.h"
#include "esp_log.h"
#include "esp_console.h"
#include "esp_idf_version.h"
#include "linenoise/linenoise.h"

#include "node_config.h"
#include "console.h"

#define CLI_PROMPT "canbus> "

static int cmd_version(int argc, char **argv)
{
	printf("canbus-esp32-idf %s (ESP-IDF %s)\n", FIRMWARE_VERSION, esp_get_idf_version());
	return 0;
}

static volatile bool cli_exit_requested;

static int cmd_exit(int argc, char **argv)
{
	cli_exit_requested = true;
	return 0;
}

static void register_core_cli_commands(void)
{
	esp_console_register_help_command();

	const esp_console_cmd_t version_cmd = {
		.command = "version",
		.help = "Show firmware version",
		.func = &cmd_version,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&version_cmd));

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
void console_init(void)
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

	register_core_cli_commands();
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
void console_task(void *arg)
{
	while (1) {
		int c = fgetc(stdin);
		if (c == '\n' || c == '\r') {
			run_cli();
		}
	}
}
