#include <string.h>
#include <stdio.h>
#include <stdbool.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/semphr.h"
#include "driver/i2c_master.h"
#include "esp_log.h"
#include "esp_console.h"

#include "node_config.h"
#include "lcd_task.h"

static const char *TAG = "lcd";

#define LCD_COLS 16
#define LCD_ROWS 2

/*
 * PCF8574 backpack bit mapping (the common one these boards ship with):
 * P0=RS, P1=R/W (always driven low -- write-only, never read the
 * HD44780 back), P2=E, P3=backlight, P4-P7=D4-D7 (upper nibble carries
 * the 4-bit data/command nibble).
 */
#define PCF_RS (1 << 0)
#define PCF_EN (1 << 2)
#define PCF_BL (1 << 3)

static i2c_master_bus_handle_t i2c_bus;
static i2c_master_dev_handle_t lcd_dev;

/* Mutex-protected "what should be shown" -- lcd_display() only ever
 * touches this; lcd_task() is the sole reader/writer of the physical
 * display. */
static struct {
	char line1[LCD_COLS + 1];
	char line2[LCD_COLS + 1];
	bool dirty;
} lcd_state;
static SemaphoreHandle_t lcd_mutex;

/* Returns whether the write actually succeeded (i2c_master_transmit(),
 * the real data-transfer call). Found by testing on real hardware
 * (2026-09-01): i2c_master_probe() reported "no response"/timeout for
 * every address on this bus even with a confirmed-good, confirmed-wired
 * backpack attached (independently verified alive at the same address
 * via a Raspberry Pi's i2cdetect) -- it's unreliable here as a presence
 * check. lcd_task_init() uses this return value, not the probe/scan, to
 * decide whether the LCD is actually present.
 *
 * Retries a few times on failure: this bus is also marginal on real
 * transmits, not just probe() -- occasional genuine "I2C software
 * timeout" errors were observed mid-write during bring-up (likely weak
 * pull-ups -- only the ESP32's internal ~45k ones are engaged -- plus
 * breadboard/jumper-wire capacitance). A single dropped nibble
 * transaction mid-byte desyncs the HD44780's 4-bit nibble pairing for
 * everything written after it, which is what garbled the screen the
 * first time this ran without retries. */
static bool pcf8574_write(uint8_t value)
{
	for (int attempt = 0; attempt < 5; attempt++) {
		if (i2c_master_transmit(lcd_dev, &value, 1, pdMS_TO_TICKS(100)) == ESP_OK) {
			return true;
		}
		/* Back-to-back retries with no gap tend to hit the same bad
		 * bus condition again -- give it a moment to settle. */
		vTaskDelay(pdMS_TO_TICKS(2));
	}
	return false;
}

/* One 4-bit nibble, latched via a rising-then-falling edge on E (the
 * HD44780 reads the data bus on E's falling edge). Backlight bit is
 * carried on every write so the backlight stays lit continuously. */
static void hd44780_write4(uint8_t nibble, bool rs)
{
	uint8_t data = (nibble & 0xF0) | PCF_BL | (rs ? PCF_RS : 0);
	pcf8574_write(data | PCF_EN);
	pcf8574_write(data);
}

static void hd44780_send(uint8_t byte, bool rs)
{
	hd44780_write4(byte & 0xF0, rs);
	hd44780_write4((byte << 4) & 0xF0, rs);
}

static void hd44780_command(uint8_t cmd)
{
	hd44780_send(cmd, false);
}

static void hd44780_data(uint8_t data)
{
	hd44780_send(data, true);
}

static void hd44780_set_cursor(uint8_t row)
{
	/* Standard HD44780 16x2 DDRAM row base addresses. */
	hd44780_command(0x80 | (row == 0 ? 0x00 : 0x40));
}

/* Space-pads to LCD_COLS so a shorter new string fully overwrites
 * whatever longer text was on that row before. */
static void hd44780_write_line(uint8_t row, const char *text)
{
	hd44780_set_cursor(row);
	size_t len = strnlen(text, LCD_COLS);
	for (size_t i = 0; i < LCD_COLS; i++) {
		hd44780_data(i < len ? (uint8_t)text[i] : ' ');
	}
}

/* Standard HD44780 4-bit-mode init sequence (Hitachi datasheet figure
 * 24) -- three blind 8-bit "function set" nibbles with specific delays
 * before the controller reliably accepts 4-bit mode, then the normal
 * function set / display on / entry mode / clear. */
static void hd44780_init_sequence(void)
{
	vTaskDelay(pdMS_TO_TICKS(50));
	hd44780_write4(0x30, false);
	vTaskDelay(pdMS_TO_TICKS(5));
	hd44780_write4(0x30, false);
	vTaskDelay(pdMS_TO_TICKS(1));
	hd44780_write4(0x30, false);
	vTaskDelay(pdMS_TO_TICKS(1));
	hd44780_write4(0x20, false); /* switch to 4-bit mode */
	vTaskDelay(pdMS_TO_TICKS(1));

	hd44780_command(0x28); /* function set: 4-bit, 2 line, 5x8 font */
	hd44780_command(0x0C); /* display on, cursor off, blink off */
	hd44780_command(0x06); /* entry mode: increment, no shift */
	hd44780_command(0x01); /* clear display */
	vTaskDelay(pdMS_TO_TICKS(2)); /* clear needs extra time */

	/* Belt and suspenders on top of the 0x01 clear command above: also
	 * explicitly write 16 spaces to each row via the same write path
	 * lcd_display() uses for real content, rather than relying only on
	 * the one-shot clear command to leave the display in a clean state.
	 * Matches lcd_state's own initial value (both lines start as empty
	 * strings), so the first real lcd_display() call is guaranteed to
	 * be writing over an actually-blank screen, not stale/garbage
	 * power-on DDRAM content. */
	hd44780_write_line(0, "");
	hd44780_write_line(1, "");
}

/* Probes every valid 7-bit I2C address (0x08-0x77 -- the range outside
 * the protocol-reserved addresses at each end) and logs what responds.
 * Informational only, for bring-up visibility (the PCF8574 backpack's
 * actual address varies by unit -- 0x27 and 0x3F are both common) --
 * NOT used to decide whether the LCD is present. i2c_master_probe() has
 * been observed on real hardware to report "no response" for every
 * address even with a confirmed-good, confirmed-wired device attached
 * (see pcf8574_write()), so lcd_task_init() gates on a real write
 * instead. */
static void i2c_scan_bus(void)
{
	int count = 0;
	for (uint8_t addr = 0x08; addr <= 0x77; addr++) {
		if (i2c_master_probe(i2c_bus, addr, pdMS_TO_TICKS(20)) == ESP_OK) {
			ESP_LOGI(TAG, "i2c scan: device found at 0x%02x", addr);
			count++;
		}
	}
	if (count == 0) {
		ESP_LOGW(TAG, "i2c scan: no devices found (probe is known "
			  "unreliable on this hardware -- not conclusive)");
	}
}

void lcd_task_init(void)
{
	lcd_mutex = xSemaphoreCreateMutex();

	i2c_master_bus_config_t bus_config = {
		.i2c_port = -1,
		.sda_io_num = I2C_SDA_GPIO,
		.scl_io_num = I2C_SCL_GPIO,
		.clk_source = I2C_CLK_SRC_DEFAULT,
		/* Higher than the typical default (7) -- more tolerance for
		 * short noise spikes on this marginal bus (see
		 * pcf8574_write()'s doc comment). */
		.glitch_ignore_cnt = 20,
		.flags.enable_internal_pullup = true,
	};
	if (i2c_new_master_bus(&bus_config, &i2c_bus) != ESP_OK) {
		ESP_LOGE(TAG, "init: failed to create I2C bus");
		return;
	}

	i2c_scan_bus(); /* informational only -- see its own doc comment */

	i2c_device_config_t dev_config = {
		.dev_addr_length = I2C_ADDR_BIT_LEN_7,
		.device_address = LCD_I2C_ADDR,
		/* Well under the 100kHz standard-mode default -- more margin
		 * for this bus's weak pull-ups (see pcf8574_write()'s doc
		 * comment). Combined with the retries there, not relied on
		 * alone. */
		.scl_speed_hz = 20000,
	};
	if (i2c_master_bus_add_device(i2c_bus, &dev_config, &lcd_dev) != ESP_OK) {
		ESP_LOGE(TAG, "init: failed to add LCD I2C device (addr 0x%02x)",
			  LCD_I2C_ADDR);
		return;
	}

	/* Presence check via a real write (see pcf8574_write()'s doc comment
	 * for why this is used instead of i2c_master_probe()). The value
	 * itself doesn't matter -- hd44780_init_sequence() immediately
	 * overwrites it as its first real step. Non-fatal: if the LCD isn't
	 * wired, this just logs and the rest of the node still boots/runs. */
	if (!pcf8574_write(0x00)) {
		ESP_LOGW(TAG, "init: no response from 0x%02x -- not wired, or "
			  "wrong LCD_I2C_ADDR (some backpacks use 0x3F)",
			  LCD_I2C_ADDR);
		return;
	}

	hd44780_init_sequence();
	ESP_LOGI(TAG, "init: display ready at 0x%02x", LCD_I2C_ADDR);
}

void lcd_display(const char *line1, const char *line2)
{
	xSemaphoreTake(lcd_mutex, portMAX_DELAY);

	char new1[LCD_COLS + 1];
	char new2[LCD_COLS + 1];
	snprintf(new1, sizeof(new1), "%s", line1);
	snprintf(new2, sizeof(new2), "%s", line2);

	if (strcmp(new1, lcd_state.line1) != 0 || strcmp(new2, lcd_state.line2) != 0) {
		memcpy(lcd_state.line1, new1, sizeof(new1));
		memcpy(lcd_state.line2, new2, sizeof(new2));
		lcd_state.dirty = true;
	}

	xSemaphoreGive(lcd_mutex);
}

void lcd_task(void *arg)
{
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(LCD_UPDATE_MS));

		xSemaphoreTake(lcd_mutex, portMAX_DELAY);
		bool dirty = lcd_state.dirty;
		char line1[LCD_COLS + 1];
		char line2[LCD_COLS + 1];
		if (dirty) {
			memcpy(line1, lcd_state.line1, sizeof(line1));
			memcpy(line2, lcd_state.line2, sizeof(line2));
			lcd_state.dirty = false;
		}
		xSemaphoreGive(lcd_mutex);

		/* Hardware write happens outside the mutex -- a concurrent
		 * lcd_display() caller on another task never blocks on this
		 * (comparatively slow) I2C transaction. */
		if (dirty) {
			hd44780_write_line(0, line1);
			hd44780_write_line(1, line2);
		}
	}
}

static int cmd_lcd(int argc, char **argv)
{
	if (argc != 3) {
		printf("usage: lcd <line1> <line2>\n");
		return 1;
	}
	lcd_display(argv[1], argv[2]);
	return 0;
}

void lcd_task_register_cli_commands(void)
{
	const esp_console_cmd_t lcd_cmd = {
		.command = "lcd",
		.help = "Show <line1> <line2> on the LCD (16 chars each, space-padded)",
		.func = &cmd_lcd,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&lcd_cmd));
}
