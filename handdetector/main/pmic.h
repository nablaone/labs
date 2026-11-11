#pragma once

#include "esp_err.h"

/* Opens the shared GPIO1/2 I2C bus (kept open for the camera's SCCB too)
 * and powers the camera rails through the AXP313A: ALDO1 = DVDD 1.2 V,
 * DLDO1 = AVDD/DOVDD 2.8 V for the OV2640. Must run before camera init --
 * without it the sensor is unpowered and SCCB probing finds nothing. */
esp_err_t pmic_camera_power_on(void);

/* Sets one camera rail in mV (ALDO1 = DVDD, DLDO1 = AVDD/DOVDD), kept
 * inside the OV2640's ratings: DVDD 1100..1400, AVDD 2500..3000. */
esp_err_t pmic_set_rail(const char *rail, int mv);

/* "i2cscan" CLI command -- lists devices on the shared bus -- plus "pmic"
 * (AXP313A register dump / rail voltage). */
void pmic_register_cli_commands(void);
