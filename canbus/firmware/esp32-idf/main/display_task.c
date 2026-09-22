#include <string.h>
#include <stdio.h>
#include <stdbool.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/i2c_master.h"
#include "esp_log.h"

#include "node_config.h"
#include "metrics.h"
#include "display_task.h"

static const char *TAG = "display";

#define LCD_COLS 16

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

/* Set only once the presence write in display_task_init() actually
 * succeeds. display_task() checks this before entering its loop and
 * deletes itself if false, rather than looping forever retrying real
 * I2C writes against hardware that was never there. */
static bool lcd_present;

/* Returns whether the write actually succeeded (i2c_master_transmit(),
 * the real data-transfer call). Found by testing on real hardware
 * (2026-09-01): i2c_master_probe() reported "no response"/timeout for
 * every address on this bus even with a confirmed-good, confirmed-wired
 * backpack attached (independently verified alive at the same address
 * via a Raspberry Pi's i2cdetect) -- it's unreliable here as a presence
 * check. display_task_init() uses this return value, not a probe, to
 * decide whether the LCD is actually present.
 *
 * Retries a few times on failure: real transmits, not just probe(),
 * occasionally hit genuine "I2C software timeout" errors mid-write. A
 * single dropped nibble transaction mid-byte desyncs the HD44780's
 * 4-bit nibble pairing for everything written after it, which is what
 * garbled the screen the first time this ran without retries. Adding
 * real external pull-up resistors (this bus initially ran on just the
 * ESP32's weak internal ~45k ones) cut the failure rate drastically,
 * confirming that was the dominant cause -- but it didn't reach zero,
 * so the retry loop stays as a safety net regardless. */
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
	 * explicitly write 16 spaces to each row, rather than relying only
	 * on the one-shot clear command to leave the display in a clean
	 * state. */
	hd44780_write_line(0, "");
	hd44780_write_line(1, "");
}

void display_task_init(void)
{
	i2c_master_bus_config_t bus_config = {
		.i2c_port = -1,
		.sda_io_num = I2C_SDA_GPIO,
		.scl_io_num = I2C_SCL_GPIO,
		.clk_source = I2C_CLK_SRC_DEFAULT,
		/* Typical default. An earlier bump to 20 (paired with a 20kHz
		 * clock below) was tried while chasing bus errors, but real
		 * external pull-ups turned out to be the actual fix -- with
		 * those in place, this standard value plus the 100kHz clock
		 * below produce noticeably *fewer* real transmit failures than
		 * the "more conservative" settings did, so this isn't a
		 * conservative-vs-aggressive tradeoff, it's just the better
		 * setting for this bus (see pcf8574_write()'s doc comment). */
		.glitch_ignore_cnt = 7,
		.flags.enable_internal_pullup = true,
	};
	if (i2c_new_master_bus(&bus_config, &i2c_bus) != ESP_OK) {
		ESP_LOGE(TAG, "init: failed to create I2C bus");
		return;
	}

	i2c_device_config_t dev_config = {
		.dev_addr_length = I2C_ADDR_BIT_LEN_7,
		.device_address = LCD_I2C_ADDR,
		/* Standard-mode default -- see glitch_ignore_cnt's comment
		 * above. */
		.scl_speed_hz = 100000,
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
	lcd_present = true;
	ESP_LOGI(TAG, "init: display ready at 0x%02x", LCD_I2C_ADDR);
}

/* Cycles through every currently registered metric, one per
 * DISPLAY_METRIC_MS: key on line 1, value on line 2. metrics_count() is
 * re-checked every iteration since the table can grow after this task
 * starts (e.g. ping_role.c/pong_role.c's metrics only appear once this
 * board's role is configured). */
void display_task(void *arg)
{
	if (!lcd_present) {
		vTaskDelete(NULL);
	}

	size_t idx = 0;
	while (1) {
		size_t count = metrics_count();
		if (count > 0) {
			char key[LCD_COLS + 1];
			char value[LCD_COLS + 1];
			if (metrics_get(idx % count, key, sizeof(key), value, sizeof(value))) {
				hd44780_write_line(0, key);
				hd44780_write_line(1, value);
			}
			idx++;
		}

		vTaskDelay(pdMS_TO_TICKS(DISPLAY_METRIC_MS));
	}
}
