#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <stddef.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include "esp_console.h"
#include "esp_log.h"
#include "esp_timer.h"

#include "node_config.h"
#include "pmic.h"
#include "camera.h"

static const char *TAG = "camera";
static bool cam_ready;

/* Guards the driver: held across grab..release and across re-inits. */
static SemaphoreHandle_t cam_lock;

/*
 * RGB565 QQVGA (160x120) by default, for the palm detector: raw frames
 * need no decode (JPEG QVGA cost ~80 ms of CPU per frame), and 160x120
 * is still more than the 96x96 model input. QVGA RGB565 is worse: the
 * camera DMA then writes ~3.4 MB/s into PSRAM, which slows the inference
 * by ~30 ms. Measured in docs/performance.md (step 12). Switch with
 * `cam fmt` / `cam res` (or the web page) at runtime, e.g. JPEG QVGA for
 * a nicer picture on the web page.
 *
 * XCLK 20 MHz is the usual OV2640 value. Two frame buffers in PSRAM let
 * DMA fill one while the app reads the other; GRAB_LATEST drops stale
 * frames so a grab always sees a fresh one.
 *
 * Not const: pixel_format/frame_size/jpeg_quality track the current mode
 * so a re-init comes back the way it was.
 */
static camera_config_t cam_config = {
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

	.pixel_format = PIXFORMAT_RGB565,
	.frame_size = FRAMESIZE_QQVGA,
	.jpeg_quality = 12,
	.fb_count = 2,
	.fb_location = CAMERA_FB_IN_PSRAM,
	.grab_mode = CAMERA_GRAB_LATEST,
	.sccb_i2c_port = I2C_BUS_PORT,
};

/* ---- name tables ---- */

static const struct { pixformat_t fmt; const char *name; } formats[] = {
	{ PIXFORMAT_JPEG, "jpeg" },
	{ PIXFORMAT_GRAYSCALE, "gray" },
	{ PIXFORMAT_RGB565, "rgb565" },
};

/* The sizes the OV2640 (2 MP, UXGA max) can actually produce. */
static const struct { framesize_t fs; const char *name; } sizes[] = {
	{ FRAMESIZE_96X96, "96x96" },
	{ FRAMESIZE_QQVGA, "qqvga" },
	{ FRAMESIZE_QCIF, "qcif" },
	{ FRAMESIZE_240X240, "240x240" },
	{ FRAMESIZE_QVGA, "qvga" },
	{ FRAMESIZE_CIF, "cif" },
	{ FRAMESIZE_VGA, "vga" },
	{ FRAMESIZE_SVGA, "svga" },
	{ FRAMESIZE_XGA, "xga" },
	{ FRAMESIZE_SXGA, "sxga" },
	{ FRAMESIZE_UXGA, "uxga" },
};

#define ARRAY_LEN(a) (sizeof(a) / sizeof((a)[0]))

int camera_format_from_name(const char *name)
{
	for (size_t i = 0; i < ARRAY_LEN(formats); i++) {
		if (strcasecmp(name, formats[i].name) == 0) return formats[i].fmt;
	}
	return -1;
}

const char *camera_format_name(pixformat_t fmt)
{
	for (size_t i = 0; i < ARRAY_LEN(formats); i++) {
		if (formats[i].fmt == fmt) return formats[i].name;
	}
	return "?";
}

int camera_framesize_from_name(const char *name)
{
	for (size_t i = 0; i < ARRAY_LEN(sizes); i++) {
		if (strcasecmp(name, sizes[i].name) == 0) return sizes[i].fs;
	}
	return -1;
}

const char *camera_framesize_name(framesize_t fs)
{
	for (size_t i = 0; i < ARRAY_LEN(sizes); i++) {
		if (sizes[i].fs == fs) return sizes[i].name;
	}
	return "?";
}

static const char *sensor_name(const sensor_t *s)
{
	for (int i = 0; i < CAMERA_MODEL_MAX; i++) {
		if (camera_sensor[i].pid == s->id.PID) {
			return camera_sensor[i].name;
		}
	}
	return "unknown";
}

/* ---- lifecycle ---- */

/*
 * This OV2640 shows horizontal row noise that scales with analog gain:
 * the colour-bar test pattern is clean, gain 0 is nearly clean, gain 30
 * is streaked across the whole frame, and nudging the AXP313A rails
 * (AVDD 3.0 V, DVDD 1.3 V) didn't change it. Left alone, AGC runs up to
 * high gain indoors, so cap it at 2x and let AEC lengthen the exposure
 * instead. Applied after every (re)init, since init resets the sensor.
 * Raise with `cam set gainceiling <0..6>` (2x..128x) in dim light.
 */
#define CAM_DEFAULT_GAINCEILING GAINCEILING_2X

/* Caller holds cam_lock. */
static esp_err_t init_locked(void)
{
	esp_err_t err = esp_camera_init(&cam_config);
	if (err != ESP_OK) {
		ESP_LOGE(TAG, "camera NOT detected: %s", esp_err_to_name(err));
		return err;
	}
	sensor_t *s = esp_camera_sensor_get();
	s->set_gainceiling(s, CAM_DEFAULT_GAINCEILING);
	cam_ready = true;
	return ESP_OK;
}

esp_err_t camera_init(void)
{
	if (!cam_lock) {
		cam_lock = xSemaphoreCreateMutex();
	}
	if (cam_ready) {
		return ESP_OK;
	}
	esp_err_t err = pmic_camera_power_on();
	if (err != ESP_OK) {
		return err;
	}

	xSemaphoreTake(cam_lock, portMAX_DELAY);
	err = init_locked();
	xSemaphoreGive(cam_lock);

	if (err == ESP_OK) {
		sensor_t *s = esp_camera_sensor_get();
		ESP_LOGI(TAG, "camera detected: %s (PID 0x%04x) at SCCB 0x%02x",
			 sensor_name(s), s->id.PID, s->slv_addr);
	}
	return err;
}

void camera_deinit(void)
{
	xSemaphoreTake(cam_lock, portMAX_DELAY);
	if (cam_ready) {
		esp_camera_deinit();
		cam_ready = false;
	}
	xSemaphoreGive(cam_lock);
}

bool camera_ready(void)
{
	return cam_ready;
}

camera_fb_t *camera_grab(void)
{
	if (!cam_lock) {
		return NULL;
	}
	xSemaphoreTake(cam_lock, portMAX_DELAY);
	camera_fb_t *fb = cam_ready ? esp_camera_fb_get() : NULL;
	if (!fb) {
		xSemaphoreGive(cam_lock);
	}
	return fb;
}

void camera_release(camera_fb_t *fb)
{
	esp_camera_fb_return(fb);
	xSemaphoreGive(cam_lock);
}

/* Re-init with the current cam_config. The sensor stays powered and the
 * I2C bus stays up, so this takes ~100 ms. */
static esp_err_t reinit(void)
{
	xSemaphoreTake(cam_lock, portMAX_DELAY);
	if (cam_ready) {
		esp_camera_deinit();
		cam_ready = false;
	}
	esp_err_t err = init_locked();
	xSemaphoreGive(cam_lock);
	return err;
}

esp_err_t camera_set_format(pixformat_t fmt)
{
	if (cam_config.pixel_format == fmt && cam_ready) return ESP_OK;
	cam_config.pixel_format = fmt;
	return reinit();
}

esp_err_t camera_set_framesize(framesize_t fs)
{
	if (cam_config.frame_size == fs && cam_ready) return ESP_OK;
	cam_config.frame_size = fs;
	return reinit();
}

esp_err_t camera_set_quality(int q)
{
	/* OV2640 JPEG quality: 0 (best) .. 63 (worst); <10 tends to
	 * overflow the frame buffer. */
	if (q < 4 || q > 63) return ESP_ERR_INVALID_ARG;
	cam_config.jpeg_quality = q;
	xSemaphoreTake(cam_lock, portMAX_DELAY);
	esp_err_t err = ESP_OK;
	if (cam_ready) {
		sensor_t *s = esp_camera_sensor_get();
		err = s->set_quality(s, q) == 0 ? ESP_OK : ESP_FAIL;
	}
	xSemaphoreGive(cam_lock);
	return err;
}

esp_err_t camera_set_xclk(int mhz)
{
	if (mhz < 5 || mhz > 24) return ESP_ERR_INVALID_ARG;
	cam_config.xclk_freq_hz = mhz * 1000000;
	return reinit();
}

/* ---- live sensor parameters ---- */

typedef int (*setter_t)(sensor_t *, int);

#define P(n, f) { n, offsetof(sensor_t, f) }
static const struct { const char *name; size_t off; } params[] = {
	P("colorbar", set_colorbar),       /* sensor test pattern */
	P("agc", set_gain_ctrl),           /* auto gain on/off */
	P("gain", set_agc_gain),           /* manual gain 0..30 (agc off) */
	P("aec", set_exposure_ctrl),       /* auto exposure on/off */
	P("exposure", set_aec_value),      /* manual exposure 0..1200 (aec off) */
	P("gainceiling", set_gainceiling), /* AGC limit 0..6 = 2x..128x */
	P("brightness", set_brightness),   /* -2..2 */
	P("contrast", set_contrast),       /* -2..2 */
	P("saturation", set_saturation),   /* -2..2 */
	P("awb", set_whitebal),
	P("bpc", set_bpc),                 /* black pixel correction */
	P("wpc", set_wpc),                 /* white pixel correction */
	P("gma", set_raw_gma),
	P("lenc", set_lenc),               /* lens correction */
	P("dcw", set_dcw),                 /* downsize enable */
	P("vflip", set_vflip),
	P("hmirror", set_hmirror),
	P("effect", set_special_effect),   /* 0 none, 2 gray, ... */
	P("aelevel", set_ae_level),        /* -2..2 */
};
#undef P

const char *camera_param_names(void)
{
	static char buf[256];
	if (!buf[0]) {
		size_t n = 0;
		for (size_t i = 0; i < ARRAY_LEN(params) && n < sizeof(buf); i++) {
			n += snprintf(buf + n, sizeof(buf) - n, "%s%s", i ? " " : "", params[i].name);
		}
	}
	return buf;
}

esp_err_t camera_set_param(const char *name, int value)
{
	for (size_t i = 0; i < ARRAY_LEN(params); i++) {
		if (strcasecmp(name, params[i].name) != 0) continue;
		xSemaphoreTake(cam_lock, portMAX_DELAY);
		esp_err_t err = ESP_ERR_INVALID_STATE;
		if (cam_ready) {
			sensor_t *s = esp_camera_sensor_get();
			setter_t fn = *(setter_t *)((char *)s + params[i].off);
			err = fn && fn(s, value) == 0 ? ESP_OK : ESP_FAIL;
		}
		xSemaphoreGive(cam_lock);
		return err;
	}
	return ESP_ERR_NOT_FOUND;
}

int camera_reg_get(int reg)
{
	xSemaphoreTake(cam_lock, portMAX_DELAY);
	int v = -1;
	if (cam_ready) {
		sensor_t *s = esp_camera_sensor_get();
		v = s->get_reg(s, reg, 0xff);
	}
	xSemaphoreGive(cam_lock);
	return v;
}

esp_err_t camera_reg_set(int reg, int value)
{
	xSemaphoreTake(cam_lock, portMAX_DELAY);
	esp_err_t err = ESP_ERR_INVALID_STATE;
	if (cam_ready) {
		sensor_t *s = esp_camera_sensor_get();
		err = s->set_reg(s, reg, 0xff, value) == 0 ? ESP_OK : ESP_FAIL;
	}
	xSemaphoreGive(cam_lock);
	return err;
}

size_t camera_info_json(char *buf, size_t len)
{
	const resolution_info_t *r = &resolution[cam_config.frame_size];
	sensor_t *s = cam_ready ? esp_camera_sensor_get() : NULL;
	int n = snprintf(buf, len,
		"{\"ready\":%s,\"sensor\":\"%s\",\"pid\":%u,\"fmt\":\"%s\",\"res\":\"%s\","
		"\"w\":%u,\"h\":%u,\"q\":%d,\"xclk\":%d}",
		cam_ready ? "true" : "false",
		s ? sensor_name(s) : "none",
		s ? s->id.PID : 0,
		camera_format_name(cam_config.pixel_format),
		camera_framesize_name(cam_config.frame_size),
		r->width, r->height, cam_config.jpeg_quality,
		cam_config.xclk_freq_hz / 1000000);
	return n < 0 ? 0 : (size_t)n < len ? (size_t)n : len - 1;
}

/* ---- CLI ---- */

static void print_info(void)
{
	sensor_t *s = cam_ready ? esp_camera_sensor_get() : NULL;
	if (!s) {
		printf("camera: not detected\n");
		return;
	}
	const resolution_info_t *r = &resolution[cam_config.frame_size];
	printf("camera: %s  PID=0x%04x VER=0x%02x MIDH=0x%02x MIDL=0x%02x SCCB=0x%02x\n",
	       sensor_name(s), s->id.PID, s->id.VER, s->id.MIDH, s->id.MIDL, s->slv_addr);
	printf("        %s %s (%ux%u), quality %d, xclk %d MHz\n",
	       camera_format_name(cam_config.pixel_format),
	       camera_framesize_name(cam_config.frame_size), r->width, r->height,
	       cam_config.jpeg_quality, cam_config.xclk_freq_hz / 1000000);
}

static void print_names(void)
{
	printf("formats:");
	for (size_t i = 0; i < ARRAY_LEN(formats); i++) printf(" %s", formats[i].name);
	printf("\nsizes:  ");
	for (size_t i = 0; i < ARRAY_LEN(sizes); i++) printf(" %s", sizes[i].name);
	printf("\n");
}

/* Prints the frame as base64 between markers, 76 chars per line, so a
 * frame can be pulled over the USB console without BLE (useful to rule
 * out radio interference, and as a fallback). */
static void dump_base64(const uint8_t *p, size_t n)
{
	static const char b64[] =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
	char line[80];
	size_t col = 0;
	for (size_t i = 0; i < n; i += 3) {
		uint32_t v = p[i] << 16 | (i + 1 < n ? p[i + 1] << 8 : 0) | (i + 2 < n ? p[i + 2] : 0);
		line[col++] = b64[(v >> 18) & 63];
		line[col++] = b64[(v >> 12) & 63];
		line[col++] = i + 1 < n ? b64[(v >> 6) & 63] : '=';
		line[col++] = i + 2 < n ? b64[v & 63] : '=';
		if (col == 76 || i + 3 >= n) {
			line[col] = '\0';
			printf("%s\n", line);
			col = 0;
		}
	}
}

static int cmd_cam(int argc, char **argv)
{
	const char *sub = argc > 1 ? argv[1] : "info";
	const char *arg = argc > 2 ? argv[2] : NULL;

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
	if (strcmp(sub, "fmt") == 0 || strcmp(sub, "res") == 0) {
		int v = !arg ? -1 : sub[0] == 'f' ? camera_format_from_name(arg)
						  : camera_framesize_from_name(arg);
		if (v < 0) {
			print_names();
			return 1;
		}
		esp_err_t err = sub[0] == 'f' ? camera_set_format(v) : camera_set_framesize(v);
		printf("%s\n", esp_err_to_name(err));
		return err == ESP_OK ? 0 : 1;
	}
	if (strcmp(sub, "q") == 0) {
		esp_err_t err = arg ? camera_set_quality(atoi(arg)) : ESP_ERR_INVALID_ARG;
		printf("%s\n", esp_err_to_name(err));
		return err == ESP_OK ? 0 : 1;
	}
	if (strcmp(sub, "set") == 0) {
		if (argc < 4) {
			printf("cam set <param> <value>; params: %s\n", camera_param_names());
			return 1;
		}
		esp_err_t err = camera_set_param(argv[2], atoi(argv[3]));
		printf("%s\n", esp_err_to_name(err));
		return err == ESP_OK ? 0 : 1;
	}
	if (strcmp(sub, "reg") == 0) {
		/* cam reg <bank:addr hex, e.g. 0x10a> [value] */
		if (argc < 3) {
			printf("cam reg <0xBAA> [value] -- B = bank (0 DSP, 1 sensor), AA = address\n");
			return 1;
		}
		int reg = strtol(argv[2], NULL, 0);
		if (argc > 3) {
			esp_err_t err = camera_reg_set(reg, strtol(argv[3], NULL, 0));
			printf("%s\n", esp_err_to_name(err));
		}
		int v = camera_reg_get(reg);
		printf("reg 0x%03x = %s0x%02x\n", reg, v < 0 ? "ERR " : "", v < 0 ? 0 : v);
		return v < 0;
	}
	if (strcmp(sub, "xclk") == 0) {
		esp_err_t err = arg ? camera_set_xclk(atoi(arg)) : ESP_ERR_INVALID_ARG;
		printf("%s\n", esp_err_to_name(err));
		return err == ESP_OK ? 0 : 1;
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
		camera_fb_t *fb = camera_grab();
		int64_t dt = esp_timer_get_time() - t0;
		if (!fb) {
			printf("capture failed\n");
			return 1;
		}
		printf("frame %ux%u, %s, %u bytes, %lld ms\n",
		       fb->width, fb->height, camera_format_name(fb->format),
		       (unsigned)fb->len, dt / 1000);
		camera_release(fb);
		return 0;
	}
	if (strcmp(sub, "bench") == 0) {
		/* Back-to-back grabs: per-call esp_camera_fb_get() latency, and
		 * the interval between sensor frame timestamps (= the real
		 * capture rate). The first 2 grabs are dropped: they return
		 * frames already sitting in the double buffer. */
		int n = arg ? atoi(arg) : 30;
		if (n < 3) n = 3;
		int64_t get_min = INT64_MAX, get_max = 0, get_sum = 0, prev_ts = 0;
		int64_t gap_min = INT64_MAX, gap_max = 0, gap_sum = 0;
		size_t bytes = 0;
		int64_t t_start = 0;
		int gaps = 0;
		for (int i = 0; i < n + 2; i++) {
			int64_t t0 = esp_timer_get_time();
			camera_fb_t *fb = camera_grab();
			int64_t dt = esp_timer_get_time() - t0;
			if (!fb) {
				printf("capture failed at %d\n", i);
				return 1;
			}
			int64_t ts = (int64_t)fb->timestamp.tv_sec * 1000000 + fb->timestamp.tv_usec;
			if (i == 2) t_start = t0;
			if (i >= 2) {
				get_min = dt < get_min ? dt : get_min;
				get_max = dt > get_max ? dt : get_max;
				get_sum += dt;
				bytes += fb->len;
			}
			if (i > 2) {
				int64_t gap = ts - prev_ts;
				gap_min = gap < gap_min ? gap : gap_min;
				gap_max = gap > gap_max ? gap : gap_max;
				gap_sum += gap;
				gaps++;
			}
			prev_ts = ts;
			camera_release(fb);
		}
		int64_t total = esp_timer_get_time() - t_start;
		printf("%s %s: %d frames in %lld ms = %.1f fps, avg %u bytes\n",
		       camera_format_name(cam_config.pixel_format),
		       camera_framesize_name(cam_config.frame_size), n, total / 1000,
		       n * 1e6 / total, (unsigned)(bytes / n));
		printf("  fb_get   min %.1f  avg %.1f  max %.1f ms\n",
		       get_min / 1e3, get_sum / 1e3 / n, get_max / 1e3);
		printf("  sensor frame interval  min %.1f  avg %.1f  max %.1f ms\n",
		       gap_min / 1e3, gap_sum / 1e3 / gaps, gap_max / 1e3);
		return 0;
	}
	if (strcmp(sub, "dump") == 0) {
		camera_fb_t *fb = camera_grab();
		if (!fb) {
			printf("capture failed\n");
			return 1;
		}
		printf("-----BEGIN FRAME %ux%u %s %u-----\n", fb->width, fb->height,
		       camera_format_name(fb->format), (unsigned)fb->len);
		dump_base64(fb->buf, fb->len);
		printf("-----END FRAME-----\n");
		camera_release(fb);
		return 0;
	}
	printf("usage: cam [info|snap|bench [n]|dump|init|deinit|fmt <f>|res <size>|q <4..63>|xclk <MHz>|set <p> <v>|reg <r> [v]]\n");
	return 1;
}

void camera_register_cli_commands(void)
{
	const esp_console_cmd_t cmd = {
		.command = "cam",
		.help = "Camera: info | snap | bench [n] | dump | init | deinit | fmt <jpeg|gray|rgb565> | res <size> | q <4..63> | xclk <MHz> | set <param> <v> | reg <r> [v]",
		.hint = "[info|snap|bench|dump|init|deinit|fmt|res|q|xclk|set|reg]",
		.func = &cmd_cam,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&cmd));
}
