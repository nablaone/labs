#include <stdio.h>
#include <string.h>

#include "esp_camera.h"
#include "esp_console.h"
#include "esp_log.h"
#include "esp_timer.h"

#include "node_config.h"
#include "pmic.h"
#include "camera.h"

static const char *TAG = "camera";
static bool cam_ready;

/*
 * JPEG QVGA to start with -- the OV2640 encodes JPEG in hardware, so it's
 * cheap and proves the full DVP pipeline works. Hand detection will
 * likely want a small RGB565/grayscale frame instead; that's just a
 * change of pixel_format/frame_size here.
 *
 * XCLK 20 MHz is the usual OV2640 value. Two frame buffers in PSRAM let
 * DMA fill one while the app reads the other; GRAB_LATEST drops stale
 * frames so `snap` always sees a fresh one.
 */
static const camera_config_t cam_config = {
	.pin_pwdn = CAM_PIN_PWDN,
	.pin_reset = CAM_PIN_RESET,
	.pin_xclk = CAM_PIN_XCLK,
	.pin_sccb_sda = -1, /* -1: reuse the bus pmic.c already opened */
	.pin_sccb_scl = -1,
	.pin_d7 = CAM_PIN_Y9,
	.pin_d6 = CAM_PIN_Y8,
	.pin_d5 = CAM_PIN_Y7,
	.pin_d4 = CAM_PIN_Y6,
	.pin_d3 = CAM_PIN_Y5,
	.pin_d2 = CAM_PIN_Y4,
	.pin_d1 = CAM_PIN_Y3,
	.pin_d0 = CAM_PIN_Y2,
	.pin_vsync = CAM_PIN_VSYNC,
	.pin_href = CAM_PIN_HREF,
	.pin_pclk = CAM_PIN_PCLK,

	.xclk_freq_hz = 20000000,
	.ledc_timer = LEDC_TIMER_0,
	.ledc_channel = LEDC_CHANNEL_0,

	.pixel_format = PIXFORMAT_JPEG,
	.frame_size = FRAMESIZE_QVGA,
	.jpeg_quality = 12,
	.fb_count = 2,
	.fb_location = CAMERA_FB_IN_PSRAM,
	.grab_mode = CAMERA_GRAB_LATEST,
	.sccb_i2c_port = I2C_BUS_PORT,
};

static const char *sensor_name(const sensor_t *s)
{
	for (int i = 0; i < CAMERA_MODEL_MAX; i++) {
		if (camera_sensor[i].pid == s->id.PID) {
			return camera_sensor[i].name;
		}
	}
	return "unknown";
}

static void print_info(void)
{
	sensor_t *s = esp_camera_sensor_get();
	if (!s) {
		printf("camera: not detected\n");
		return;
	}
	printf("camera: %s  PID=0x%04x VER=0x%02x MIDH=0x%02x MIDL=0x%02x SCCB=0x%02x\n",
	       sensor_name(s), s->id.PID, s->id.VER, s->id.MIDH, s->id.MIDL, s->slv_addr);
	printf("        xclk %d MHz, format %d, framesize %d, quality %d\n",
	       s->xclk_freq_hz / 1000000, s->pixformat, s->status.framesize, s->status.quality);
}

esp_err_t camera_init(void)
{
	if (cam_ready) {
		return ESP_OK;
	}
	esp_err_t err = pmic_camera_power_on();
	if (err != ESP_OK) {
		return err;
	}
	err = esp_camera_init(&cam_config);
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "camera NOT detected: %s", esp_err_to_name(err));
		return err;
	}
	cam_ready = true;

	sensor_t *s = esp_camera_sensor_get();
	ESP_LOGI(TAG, "camera detected: %s (PID 0x%04x) at SCCB 0x%02x",
		 sensor_name(s), s->id.PID, s->slv_addr);
	return ESP_OK;
}

void camera_deinit(void)
{
	if (cam_ready) {
		esp_camera_deinit();
		cam_ready = false;
	}
}

bool camera_ready(void)
{
	return cam_ready;
}

static int cmd_cam(int argc, char **argv)
{
	const char *sub = argc > 1 ? argv[1] : "info";

	if (strcmp(sub, "init") == 0) {
		esp_err_t err = camera_init();
		printf("init: %s\n", esp_err_to_name(err));
		if (err == ESP_OK) print_info();
		return err == ESP_OK ? 0 : 1;
	}
	if (strcmp(sub, "deinit") == 0) {
		camera_deinit();
		printf("camera released\n");
		return 0;
	}
	if (!cam_ready) {
		printf("camera not initialised -- try 'cam init'\n");
		return 1;
	}
	if (strcmp(sub, "info") == 0) {
		print_info();
		return 0;
	}
	if (strcmp(sub, "snap") == 0) {
		int64_t t0 = esp_timer_get_time();
		camera_fb_t *fb = esp_camera_fb_get();
		int64_t dt = esp_timer_get_time() - t0;
		if (!fb) {
			printf("capture failed\n");
			return 1;
		}
		printf("frame %ux%u, format %d, %u bytes, %lld ms\n",
		       fb->width, fb->height, fb->format, (unsigned)fb->len, dt / 1000);
		esp_camera_fb_return(fb);
		return 0;
	}
	printf("usage: cam [info|snap|init|deinit]\n");
	return 1;
}

void camera_register_cli_commands(void)
{
	const esp_console_cmd_t cmd = {
		.command = "cam",
		.help = "Camera: info | snap | init | deinit",
		.hint = "[info|snap|init|deinit]",
		.func = &cmd_cam,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&cmd));
}
