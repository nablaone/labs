#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/i2c_master.h"
#include "esp_console.h"
#include "esp_log.h"

#include "node_config.h"
#include "pmic.h"

static const char *TAG = "pmic";

/*
 * AXP313A registers, as used by DFRobot's own DFRobot_AXP313A Arduino
 * library (github.com/cdjq/DFRobot_AXP313A):
 *   0x10 output enable bits -- 0x19 = DCDC1 + ALDO1 + DLDO1 on
 *   0x16 ALDO1 voltage      -- (V - 0.5) * 10, 0.1 V steps
 *   0x17 DLDO1 voltage      -- same encoding
 * DCDC1 is the board's 3V3 rail, so it must stay on.
 */
#define AXP_REG_OUTPUT_CTRL 0x10
#define AXP_REG_ALDO1_V     0x16
#define AXP_REG_DLDO1_V     0x17
#define AXP_OUTPUTS_ON      0x19
#define AXP_MV(mv)          ((uint8_t)(((mv) - 500) / 100))

static i2c_master_bus_handle_t bus;

/*
 * One bus for the whole firmware, opened once and never torn down:
 * esp32-camera looks it up by port number (i2c_master_get_bus_handle)
 * and adds the OV2640 as another device on it.
 */
static esp_err_t bus_open(void)
{
	if (bus) {
		return ESP_OK;
	}
	const i2c_master_bus_config_t cfg = {
		.i2c_port = I2C_BUS_PORT,
		.sda_io_num = I2C_SDA_GPIO,
		.scl_io_num = I2C_SCL_GPIO,
		.clk_source = I2C_CLK_SRC_DEFAULT,
		.glitch_ignore_cnt = 7,
		.flags.enable_internal_pullup = true,
	};
	return i2c_new_master_bus(&cfg, &bus);
}

static esp_err_t axp_write(i2c_master_dev_handle_t dev, uint8_t reg, uint8_t val)
{
	uint8_t buf[2] = { reg, val };
	esp_err_t err = i2c_master_transmit(dev, buf, sizeof(buf), 100);
	vTaskDelay(pdMS_TO_TICKS(10));
	return err;
}

esp_err_t pmic_camera_power_on(void)
{
	esp_err_t err = bus_open();
	if (err != ESP_OK) {
		return err;
	}

	err = i2c_master_probe(bus, AXP313A_ADDR, 100);
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "AXP313A not found at 0x%02x: %s", AXP313A_ADDR, esp_err_to_name(err));
		return err;
	}

	const i2c_device_config_t dev_cfg = {
		.dev_addr_length = I2C_ADDR_BIT_LEN_7,
		.device_address = AXP313A_ADDR,
		.scl_speed_hz = 100000,
	};
	i2c_master_dev_handle_t dev;
	ESP_ERROR_CHECK(i2c_master_bus_add_device(bus, &dev_cfg, &dev));

	err = axp_write(dev, AXP_REG_OUTPUT_CTRL, AXP_OUTPUTS_ON);
	if (err == ESP_OK) err = axp_write(dev, AXP_REG_ALDO1_V, AXP_MV(1200));
	if (err == ESP_OK) err = axp_write(dev, AXP_REG_DLDO1_V, AXP_MV(2800));

	i2c_master_bus_rm_device(dev);

	if (err == ESP_OK) {
		ESP_LOGI(TAG, "camera power on (DVDD 1.2 V, AVDD/DOVDD 2.8 V)");
		/* Let the rails settle before the sensor is clocked. */
		vTaskDelay(pdMS_TO_TICKS(50));
	} else {
		ESP_LOGE(TAG, "AXP313A write failed: %s", esp_err_to_name(err));
	}
	return err;
}

static i2c_master_dev_handle_t axp_dev(void)
{
	static i2c_master_dev_handle_t dev;
	if (!dev && bus_open() == ESP_OK) {
		const i2c_device_config_t cfg = {
			.dev_addr_length = I2C_ADDR_BIT_LEN_7,
			.device_address = AXP313A_ADDR,
			.scl_speed_hz = 100000,
		};
		if (i2c_master_bus_add_device(bus, &cfg, &dev) != ESP_OK) dev = NULL;
	}
	return dev;
}

static int axp_read(uint8_t reg)
{
	uint8_t v;
	i2c_master_dev_handle_t dev = axp_dev();
	if (!dev || i2c_master_transmit_receive(dev, &reg, 1, &v, 1, 100) != ESP_OK) return -1;
	return v;
}

esp_err_t pmic_set_rail(const char *rail, int mv)
{
	uint8_t reg;
	if (strcmp(rail, "aldo") == 0 && mv >= 1100 && mv <= 1400) reg = AXP_REG_ALDO1_V;
	else if (strcmp(rail, "dldo") == 0 && mv >= 2500 && mv <= 3000) reg = AXP_REG_DLDO1_V;
	else return ESP_ERR_INVALID_ARG;
	i2c_master_dev_handle_t dev = axp_dev();
	return dev ? axp_write(dev, reg, AXP_MV(mv)) : ESP_FAIL;
}

/* pmic                 dump registers 0x00..0x20
 * pmic aldo|dldo <mV>  set a camera rail */
static int cmd_pmic(int argc, char **argv)
{
	if (argc >= 3) {
		esp_err_t err = pmic_set_rail(argv[1], atoi(argv[2]));
		printf("%s\n", esp_err_to_name(err));
		if (err != ESP_OK) printf("aldo 1100..1400 mV (DVDD), dldo 2500..3000 mV (AVDD/DOVDD)\n");
		return err != ESP_OK;
	}
	for (int r = 0; r <= 0x20; r++) {
		int v = axp_read(r);
		if (v < 0) { printf("read 0x%02x failed\n", r); return 1; }
		printf("%02x:%02x%s", r, v, (r % 8 == 7) ? "\n" : "  ");
	}
	int a = axp_read(AXP_REG_ALDO1_V), d = axp_read(AXP_REG_DLDO1_V);
	printf("\noutputs 0x%02x, ALDO1 %d mV, DLDO1 %d mV\n", axp_read(AXP_REG_OUTPUT_CTRL),
	       500 + (a & 0x1f) * 100, 500 + (d & 0x1f) * 100);
	return 0;
}

static int cmd_i2cscan(int argc, char **argv)
{
	esp_err_t err = bus_open();
	if (err != ESP_OK) {
		printf("bus open failed: %s\n", esp_err_to_name(err));
		return 1;
	}
	int found = 0;
	for (uint8_t a = 0x08; a < 0x78; a++) {
		if (i2c_master_probe(bus, a, 20) == ESP_OK) {
			printf("  0x%02x%s\n", a,
			       a == AXP313A_ADDR ? "  AXP313A" : a == 0x30 ? "  OV2640 (SCCB)" : "");
			found++;
		}
	}
	printf("%d device(s)\n", found);
	return 0;
}

void pmic_register_cli_commands(void)
{
	const esp_console_cmd_t cmd = {
		.command = "i2cscan",
		.help = "Scan the shared GPIO1/2 I2C bus (PMIC + camera)",
		.func = &cmd_i2cscan,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&cmd));

	const esp_console_cmd_t pmic_cmd = {
		.command = "pmic",
		.help = "AXP313A: dump registers, or 'pmic aldo|dldo <mV>' to set a camera rail",
		.func = &cmd_pmic,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&pmic_cmd));
}
