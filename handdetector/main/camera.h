#pragma once

#include <stdbool.h>
#include "esp_err.h"

/* Initialises the OV2640 through esp32-camera (powers it via the PMIC
 * first). Probing the sensor ID over SCCB is how the camera is
 * "detected": a failure here means no camera / no power / bad ribbon. */
esp_err_t camera_init(void);
void camera_deinit(void);
bool camera_ready(void);

/* "cam" CLI command: info | snap | init | deinit. */
void camera_register_cli_commands(void);
