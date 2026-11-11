#pragma once

#include <stdbool.h>
#include <stddef.h>
#include "esp_err.h"
#include "esp_camera.h"

#ifdef __cplusplus
extern "C" {
#endif

/* Initialises the OV2640 through esp32-camera (powers it via the PMIC
 * first). Probing the sensor ID over SCCB is how the camera is
 * "detected": a failure here means no camera / no power / bad ribbon. */
esp_err_t camera_init(void);
void camera_deinit(void);
bool camera_ready(void);

/*
 * Frame access for every consumer (CLI, BLE link, later the detector).
 * camera_grab() takes the camera lock and holds it until
 * camera_release(), so a mode change can't pull the frame buffers out
 * from under a reader. Returns NULL (lock not held) if the camera isn't
 * up or the capture failed.
 */
camera_fb_t *camera_grab(void);
void camera_release(camera_fb_t *fb);

/*
 * Mode changes. Pixel format and frame size need a driver re-init
 * (frame buffers are sized at init); JPEG quality is a live sensor
 * register write.
 */
esp_err_t camera_set_format(pixformat_t fmt);
esp_err_t camera_set_framesize(framesize_t fs);
esp_err_t camera_set_quality(int q);
esp_err_t camera_set_xclk(int mhz);   /* re-init; 5..24 MHz */

/*
 * Live sensor control by name -- colorbar, agc, gain, aec, exposure,
 * gainceiling, brightness, contrast, saturation, awb, bpc, wpc, gma,
 * lenc, dcw, vflip, hmirror, effect, aelevel. Also "reg" for raw OV2640
 * registers: name "reg", value = (bank << 8 | addr) << 8 | data -- easier
 * to use through camera_reg_get/set below.
 * Returns ESP_ERR_NOT_FOUND for an unknown name.
 */
esp_err_t camera_set_param(const char *name, int value);
/* Space-separated list of the names above, for help text. */
const char *camera_param_names(void);
/* Raw OV2640 register access: reg = bank << 8 | addr (bank 0 = DSP,
 * 1 = sensor). get returns <0 on error. */
int camera_reg_get(int reg);
esp_err_t camera_reg_set(int reg, int value);

/* Name <-> enum helpers shared by the CLI and the BLE command parser.
 * Return -1 / "?" for unknown values. */
int camera_format_from_name(const char *name);
const char *camera_format_name(pixformat_t fmt);
int camera_framesize_from_name(const char *name);
const char *camera_framesize_name(framesize_t fs);

/* Current state as a compact JSON object (no trailing newline). */
size_t camera_info_json(char *buf, size_t len);

/* "cam" CLI command: info | snap | init | deinit | fmt | res | q. */
void camera_register_cli_commands(void);

#ifdef __cplusplus
}
#endif
