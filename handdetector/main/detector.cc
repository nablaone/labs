#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/semphr.h"
#include "esp_console.h"
#include "esp_heap_caps.h"
#include "esp_log.h"
#include "esp_partition.h"
#include "esp_timer.h"
#include "img_converters.h"

#include "tensorflow/lite/micro/micro_interpreter.h"
#include "tensorflow/lite/micro/micro_mutable_op_resolver.h"
#include "tensorflow/lite/micro/micro_profiler_interface.h"
#include "tensorflow/lite/micro/schema/schema_generated.h"

#include "camera.h"
#include "pie_kernels.h"
#include "pie_parallel.h"
#include "pie_fusion.h"
#include "heartbeat_task.h"
#include "detector.h"

static const char *TAG = "detector";

/* Data partition subtype for the model (custom range 0x40..0xFE). */
#define MODEL_PARTITION_SUBTYPE 0x40
#define MODEL_PARTITION_LABEL   "model"

/* TFLM working memory: every intermediate tensor lives here. The 192x192
 * model peaks well above internal SRAM, so it goes to PSRAM. The real
 * need is logged after AllocateTensors() (det: "arena used"). */
#define ARENA_SIZE (2 * 1024 * 1024)

/* Source frame as RGB888, QVGA max (decoded JPEG or unpacked RGB565). */
#define SRC_MAX_W 320
#define SRC_MAX_H 240

/* SSD anchors, MediaPipe palm config: strides 8 and 16 with 2 and 6
 * anchors per cell (see model/pipeline/common.py). */
static const struct { int stride, per_cell; } ANCHOR_LAYOUT[] = { { 8, 2 }, { 16, 6 } };

/* Host test vectors live 3 MB into the model partition (model/Makefile
 * flash-golden): "GOLD", u32 lengths of input/boxes/scores, the tensors. */
#define GOLDEN_OFFSET 0x300000

static const tflite::Model *model;
static const uint8_t *partition_base;
static SemaphoreHandle_t infer_lock;  /* detector task vs `det golden` */
static tflite::MicroInterpreter *interpreter;
static TfLiteTensor *t_in, *t_boxes, *t_scores;
static int res;          /* model input is res x res x 3 */
static int n_anchors;
static float *anchors;   /* n_anchors x (cx, cy), normalized */
static uint8_t *rgb;     /* SRC_MAX_W * SRC_MAX_H * 3 */
static int8_t q_lut[256];/* uint8 pixel -> quantized model input */

/*
 * Per-operator timing. TFLM calls BeginEvent/EndEvent around every
 * kernel's Invoke with the op name as tag (a static string, so the
 * pointer identifies the op type). Totals are per op type over the last
 * inference; `det prof` prints them.
 */
class OpProfiler : public tflite::MicroProfilerInterface {
 public:
	static constexpr int kMaxTags = 24;
	struct Row { const char *tag; uint32_t count; int64_t us; };
	Row rows[kMaxTags];
	int n_rows = 0;

	void Reset() { n_rows = 0; }

	uint32_t BeginEvent(const char *tag) override
	{
		int i = 0;
		while (i < n_rows && rows[i].tag != tag) i++;
		if (i == n_rows && n_rows < kMaxTags) rows[n_rows++] = { tag, 0, 0 };
		start_ = esp_timer_get_time();
		return (uint32_t)i;
	}

	void EndEvent(uint32_t i) override
	{
		if (i >= (uint32_t)n_rows) return;
		rows[i].count++;
		rows[i].us += esp_timer_get_time() - start_;
	}

 private:
	int64_t start_ = 0;  /* kernels don't nest */
};

static OpProfiler profiler;

static volatile bool enabled;
static float threshold = 0.5f;

/* Snapshot of the last inference (see detector.h). `pending_in` is the
 * input copied before Invoke -- TFLM's planner may reuse the input
 * tensor's memory for intermediates -- and published with the outputs
 * afterwards under snap_lock. */
struct __attribute__((packed)) SnapHeader {
	char magic[4];
	uint16_t res, n_anchors, src_w, src_h;
	float lb_ratio;
	int16_t lb_left, lb_top;
	float in_scale; int32_t in_zp;
	float box_scale; int32_t box_zp;
	float score_scale; int32_t score_zp;
	float threshold;
	uint32_t seq, infer_ms;
	uint8_t source, pad[3];
};
static_assert(sizeof(SnapHeader) == 60, "snapshot header layout");
static SemaphoreHandle_t snap_lock;
static SnapHeader snap_hdr;
static bool snap_valid;
static int8_t *pending_in, *snap_in, *snap_scores, *snap_boxes;
static uint32_t snap_seq;

static struct {
	uint32_t frames, hits;
	float score;                 /* best score of the last frame */
	float box[4];                /* x1 y1 x2 y2, source pixels */
	int above;                   /* anchors above threshold */
	uint32_t t_grab, t_pre, t_infer, t_post; /* us */
	size_t arena_used;
	char error[64];
} st;

static float sigmoidf(float x) { return 1.0f / (1.0f + expf(-x)); }

static float dequant(const TfLiteTensor *t, int8_t v)
{
	return (v - t->params.zero_point) * t->params.scale;
}

/* ---- setup ---- */

static bool map_model(void)
{
	const esp_partition_t *part = esp_partition_find_first(
		ESP_PARTITION_TYPE_DATA, (esp_partition_subtype_t)MODEL_PARTITION_SUBTYPE, MODEL_PARTITION_LABEL);
	if (!part) {
		snprintf(st.error, sizeof(st.error), "no '%s' partition", MODEL_PARTITION_LABEL);
		return false;
	}
	const void *ptr;
	esp_partition_mmap_handle_t h;
	if (esp_partition_mmap(part, 0, part->size, ESP_PARTITION_MMAP_DATA, &ptr, &h) != ESP_OK) {
		snprintf(st.error, sizeof(st.error), "mmap failed");
		return false;
	}
	/* A flatbuffer .tflite carries the identifier "TFL3" at offset 4;
	 * an erased partition reads 0xFF. */
	if (memcmp((const uint8_t *)ptr + 4, "TFL3", 4) != 0) {
		snprintf(st.error, sizeof(st.error), "no model flashed (make -C model flash-model)");
		return false;
	}
	partition_base = (const uint8_t *)ptr;
	model = tflite::GetModel(ptr);
	if (model->version() != TFLITE_SCHEMA_VERSION) {
		snprintf(st.error, sizeof(st.error), "schema %lu != %d",
			 (unsigned long)model->version(), TFLITE_SCHEMA_VERSION);
		return false;
	}
	pie_fusion_set_model(model);  /* tensor consumer counts for op fusion */
	ESP_LOGI(TAG, "model mapped from flash 0x%lx (%lu KB partition)",
		 (unsigned long)part->address, (unsigned long)part->size / 1024);
	return true;
}

static bool make_anchors(void)
{
	n_anchors = 0;
	for (auto &l : ANCHOR_LAYOUT) n_anchors += (res / l.stride) * (res / l.stride) * l.per_cell;
	anchors = (float *)heap_caps_malloc(n_anchors * 2 * sizeof(float), MALLOC_CAP_SPIRAM);
	if (!anchors) return false;
	int i = 0;
	for (auto &l : ANCHOR_LAYOUT) {
		int f = res / l.stride;
		for (int y = 0; y < f; y++)
			for (int x = 0; x < f; x++)
				for (int k = 0; k < l.per_cell; k++) {
					anchors[2 * i] = (x + 0.5f) / f;
					anchors[2 * i + 1] = (y + 0.5f) / f;
					i++;
				}
	}
	return true;
}

void detector_init(void)
{
	infer_lock = xSemaphoreCreateMutex();
	pie_parallel_init();  /* kernel worker tasks, one per core */
	if (!map_model()) {
		ESP_LOGW(TAG, "disabled: %s", st.error);
		return;
	}

	/* Exactly the operators convert.py reports for the int8 model; an
	 * op missing here fails AllocateTensors() with its name logged.
	 * ABS/MUL/RELU/SUB come from onnx2tf's tf_converter path
	 * decomposing PRelu: prelu(x) = relu(x) - alpha * relu(-x) style. */
	static tflite::MicroMutableOpResolver<14> ops;
	ops.AddAbs(PieRegister_ABS());  /* lookup tables, see pie_elementwise.cc */
	ops.AddAdd(PieRegister_ADD());
	ops.AddConcatenation();
	ops.AddConv2D(PieRegister_CONV_2D());  /* PIE SIMD paths, see pie_conv.cc */
	ops.AddDepthwiseConv2D(PieRegister_DEPTHWISE_CONV_2D());
	ops.AddMaxPool2D();
	ops.AddMul(PieRegister_MUL());
	ops.AddPad();
	ops.AddPrelu(PieRegister_PRELU());  /* native PRELU (flatbuffer_direct models) */
	ops.AddRelu(PieRegister_RELU());
	ops.AddReshape();
	ops.AddResizeBilinear();
	ops.AddSub(PieRegister_SUB());
	ops.AddTranspose();

	uint8_t *arena = (uint8_t *)heap_caps_aligned_alloc(16, ARENA_SIZE, MALLOC_CAP_SPIRAM);
	rgb = (uint8_t *)heap_caps_malloc(SRC_MAX_W * SRC_MAX_H * 3, MALLOC_CAP_SPIRAM);
	if (!arena || !rgb) {
		snprintf(st.error, sizeof(st.error), "out of PSRAM");
		ESP_LOGE(TAG, "%s", st.error);
		return;
	}

	static tflite::MicroInterpreter interp(model, ops, arena, ARENA_SIZE, nullptr, &profiler);
	if (interp.AllocateTensors() != kTfLiteOk) {
		snprintf(st.error, sizeof(st.error), "AllocateTensors failed");
		ESP_LOGE(TAG, "%s", st.error);
		return;
	}
	interpreter = &interp;
	st.arena_used = interp.arena_used_bytes();

	t_in = interp.input(0);
	/* Outputs by shape: [1, N, 4] boxes, [1, N, 1] scores. */
	for (size_t i = 0; i < interp.outputs_size(); i++) {
		TfLiteTensor *t = interp.output(i);
		if (t->dims->data[t->dims->size - 1] == 4) t_boxes = t;
		else t_scores = t;
	}
	if (t_in->type != kTfLiteInt8 || !t_boxes || !t_scores ||
	    t_boxes->type != kTfLiteInt8 || t_scores->type != kTfLiteInt8 ||
	    t_in->dims->size != 4 || t_in->dims->data[1] != t_in->dims->data[2]) {
		snprintf(st.error, sizeof(st.error), "unexpected model I/O");
		ESP_LOGE(TAG, "%s", st.error);
		interpreter = nullptr;
		return;
	}
	res = t_in->dims->data[1];
	if (!make_anchors() || t_scores->dims->data[1] != n_anchors) {
		snprintf(st.error, sizeof(st.error), "anchor count mismatch");
		ESP_LOGE(TAG, "%s", st.error);
		interpreter = nullptr;
		return;
	}

	size_t in_bytes = (size_t)res * res * 3;
	pending_in = (int8_t *)heap_caps_malloc(in_bytes, MALLOC_CAP_SPIRAM);
	snap_in = (int8_t *)heap_caps_malloc(in_bytes, MALLOC_CAP_SPIRAM);
	snap_scores = (int8_t *)heap_caps_malloc(n_anchors, MALLOC_CAP_SPIRAM);
	snap_boxes = (int8_t *)heap_caps_malloc(n_anchors * 4, MALLOC_CAP_SPIRAM);
	snap_lock = xSemaphoreCreateMutex();

	/* Model input is pixel/255 quantized with the input tensor's scale
	 * and zero point; a 256-entry table does it per byte. */
	for (int v = 0; v < 256; v++) {
		int q = (int)lroundf(v / 255.0f / t_in->params.scale) + t_in->params.zero_point;
		q_lut[v] = (int8_t)(q < -128 ? -128 : q > 127 ? 127 : q);
	}

	st.error[0] = '\0';
	enabled = true;
	ESP_LOGI(TAG, "ready: input %dx%d int8, %d anchors, arena %u KB used",
		 res, res, n_anchors, (unsigned)(st.arena_used / 1024));
}

/* ---- per frame ---- */

/* Frame -> RGB888 in `rgb`. Note the channel orders differ in
 * esp32-camera: its JPEG path (TJpgDec) yields R,G,B, while its RGB565
 * path yields B,G,R (BMP order) -- so RGB565 is unpacked here instead.
 * Camera RGB565 is big-endian. */
static bool to_rgb(const camera_fb_t *fb)
{
	if (fb->width > SRC_MAX_W || fb->height > SRC_MAX_H) return false;
	if (fb->format == PIXFORMAT_JPEG) {
		return fmt2rgb888_bounded(fb->buf, fb->len, PIXFORMAT_JPEG, rgb, SRC_MAX_W * SRC_MAX_H * 3);
	}
	if (fb->format == PIXFORMAT_RGB565) {
		const uint8_t *s = fb->buf;
		uint8_t *d = rgb;
		for (size_t i = 0; i < (size_t)fb->width * fb->height; i++, s += 2) {
			uint16_t v = (s[0] << 8) | s[1];
			*d++ = (v >> 8) & 0xf8;
			*d++ = (v >> 3) & 0xfc;
			*d++ = (v << 3) & 0xf8;
		}
		return true;
	}
	return false;
}

struct Letterbox { float ratio; int left, top; };

/* Aspect-preserving bilinear resize of the w x h RGB frame into the
 * centre of the res x res int8 input, black padding -- the same geometry
 * as model/pipeline/common.py letterbox(). */
static Letterbox preprocess(int w, int h)
{
	Letterbox lb;
	lb.ratio = fminf((float)res / w, (float)res / h);
	int nw = lroundf(w * lb.ratio), nh = lroundf(h * lb.ratio);
	lb.left = (res - nw) / 2;
	lb.top = (res - nh) / 2;

	int8_t *dst = t_in->data.int8;
	const int8_t pad = q_lut[0];
	memset(dst, pad, (size_t)res * res * 3);
	const float inv = 1.0f / lb.ratio;
	for (int y = 0; y < nh; y++) {
		/* pixel-centre mapping, like cv2.resize INTER_LINEAR */
		float sy = (y + 0.5f) * inv - 0.5f;
		int y0 = sy < 0 ? 0 : (int)sy;
		int y1 = y0 + 1 < h ? y0 + 1 : h - 1;
		float fy = sy - y0; if (fy < 0) fy = 0;
		int8_t *row = dst + ((size_t)(y + lb.top) * res + lb.left) * 3;
		for (int x = 0; x < nw; x++) {
			float sx = (x + 0.5f) * inv - 0.5f;
			int x0 = sx < 0 ? 0 : (int)sx;
			int x1 = x0 + 1 < w ? x0 + 1 : w - 1;
			float fx = sx - x0; if (fx < 0) fx = 0;
			const uint8_t *p00 = rgb + ((size_t)y0 * w + x0) * 3, *p01 = rgb + ((size_t)y0 * w + x1) * 3;
			const uint8_t *p10 = rgb + ((size_t)y1 * w + x0) * 3, *p11 = rgb + ((size_t)y1 * w + x1) * 3;
			for (int c = 0; c < 3; c++) {
				float top = p00[c] + (p01[c] - p00[c]) * fx;
				float bot = p10[c] + (p11[c] - p10[c]) * fx;
				*row++ = q_lut[(int)(top + (bot - top) * fy + 0.5f)];
			}
		}
	}
	return lb;
}

/* Best anchor only: the LED needs "is there a hand", and one box is
 * enough to show where. Thresholding happens on the raw int8 logit. */
static void postprocess(const Letterbox &lb)
{
	const int8_t *s = t_scores->data.int8;
	int best = 0, above = 0;
	float logit_thr = logf(threshold / (1.0f - threshold));
	for (int i = 0; i < n_anchors; i++) {
		if (s[i] > s[best]) best = i;
		if (dequant(t_scores, s[i]) >= logit_thr) above++;
	}
	st.above = above;
	st.score = sigmoidf(dequant(t_scores, s[best]));

	const int8_t *b = t_boxes->data.int8 + best * 4;
	float cx = dequant(t_boxes, b[0]) / res + anchors[2 * best];
	float cy = dequant(t_boxes, b[1]) / res + anchors[2 * best + 1];
	float bw = dequant(t_boxes, b[2]) / res, bh = dequant(t_boxes, b[3]) / res;
	st.box[0] = ((cx - bw / 2) * res - lb.left) / lb.ratio;
	st.box[1] = ((cy - bh / 2) * res - lb.top) / lb.ratio;
	st.box[2] = ((cx + bw / 2) * res - lb.left) / lb.ratio;
	st.box[3] = ((cy + bh / 2) * res - lb.top) / lb.ratio;
}

static void publish_snapshot(const Letterbox &lb, int src_w, int src_h, uint32_t infer_ms, uint8_t source)
{
	if (!snap_lock || !snap_in) return;
	xSemaphoreTake(snap_lock, portMAX_DELAY);
	memcpy(snap_in, pending_in, (size_t)res * res * 3);
	memcpy(snap_scores, t_scores->data.int8, n_anchors);
	memcpy(snap_boxes, t_boxes->data.int8, (size_t)n_anchors * 4);
	SnapHeader &h = snap_hdr;
	memcpy(h.magic, "DET1", 4);
	h.res = res; h.n_anchors = n_anchors; h.src_w = src_w; h.src_h = src_h;
	h.lb_ratio = lb.ratio; h.lb_left = lb.left; h.lb_top = lb.top;
	h.in_scale = t_in->params.scale; h.in_zp = t_in->params.zero_point;
	h.box_scale = t_boxes->params.scale; h.box_zp = t_boxes->params.zero_point;
	h.score_scale = t_scores->params.scale; h.score_zp = t_scores->params.zero_point;
	h.threshold = threshold;
	h.seq = ++snap_seq; h.infer_ms = infer_ms; h.source = source;
	snap_valid = true;
	xSemaphoreGive(snap_lock);
}

size_t detector_snapshot_size(void)
{
	return snap_valid ? sizeof(SnapHeader) + (size_t)res * res * 3 + n_anchors * 5 : 0;
}

size_t detector_snapshot(uint8_t *buf, size_t cap)
{
	size_t need = detector_snapshot_size();
	if (!need || cap < need) return 0;
	xSemaphoreTake(snap_lock, portMAX_DELAY);
	uint8_t *p = buf;
	memcpy(p, &snap_hdr, sizeof(snap_hdr)); p += sizeof(snap_hdr);
	memcpy(p, snap_in, (size_t)res * res * 3); p += (size_t)res * res * 3;
	memcpy(p, snap_scores, n_anchors); p += n_anchors;
	memcpy(p, snap_boxes, (size_t)n_anchors * 4);
	xSemaphoreGive(snap_lock);
	return need;
}

static void run_once(void)
{
	int64_t t0 = esp_timer_get_time();
	camera_fb_t *fb = camera_grab();
	if (!fb) {
		vTaskDelay(pdMS_TO_TICKS(100));
		return;
	}
	int w = fb->width, h = fb->height;
	bool ok = to_rgb(fb);
	camera_release(fb);
	int64_t t1 = esp_timer_get_time();
	if (!ok) {
		snprintf(st.error, sizeof(st.error), "unsupported frame (use jpeg/rgb565, <= qvga)");
		vTaskDelay(pdMS_TO_TICKS(500));
		return;
	}
	st.error[0] = '\0';

	xSemaphoreTake(infer_lock, portMAX_DELAY);
	Letterbox lb = preprocess(w, h);
	memcpy(pending_in, t_in->data.int8, (size_t)res * res * 3);
	profiler.Reset();
	pie_elementwise_reset_stats();
	pie_parallel_reset_stats();
	int64_t t2 = esp_timer_get_time();
	TfLiteStatus rc = interpreter->Invoke();
	int64_t t3 = esp_timer_get_time();
	if (rc != kTfLiteOk) {
		xSemaphoreGive(infer_lock);
		snprintf(st.error, sizeof(st.error), "Invoke failed");
		vTaskDelay(pdMS_TO_TICKS(500));
		return;
	}
	postprocess(lb);
	publish_snapshot(lb, w, h, (t3 - t2) / 1000, 0);
	xSemaphoreGive(infer_lock);
	int64_t t4 = esp_timer_get_time();

	st.t_grab = t1 - t0; st.t_pre = t2 - t1; st.t_infer = t3 - t2; st.t_post = t4 - t3;
	st.frames++;
	bool hand = st.score >= threshold;
	if (hand) st.hits++;
	heartbeat_set_fast(hand);
	ESP_LOGI(TAG, "%s score %.2f (%d anchors) | grab %lu pre %lu infer %lu post %lu ms",
		 hand ? "HAND" : "none", st.score, st.above,
		 st.t_grab / 1000, st.t_pre / 1000, st.t_infer / 1000, st.t_post / 1000);
}

void detector_task(void *arg)
{
	while (1) {
		if (!enabled || !interpreter) {
			heartbeat_set_fast(false);
			vTaskDelay(pdMS_TO_TICKS(500));
			continue;
		}
		run_once();
	}
}

/* ---- reporting ---- */

uint32_t detector_last(float *score, bool *hand)
{
	if (score) *score = st.score;
	if (hand) *hand = st.score >= threshold;
	return snap_seq;
}

size_t detector_info_json(char *buf, size_t len)
{
	int n = snprintf(buf, len,
		"{\"on\":%s,\"res\":%d,\"thr\":%.2f,\"seq\":%lu,\"frames\":%lu,\"hits\":%lu,\"score\":%.3f,"
		"\"box\":[%.0f,%.0f,%.0f,%.0f],\"infer_ms\":%lu,\"pre_ms\":%lu,\"err\":\"%.40s\"}",
		enabled ? "true" : "false", res, threshold, (unsigned long)snap_seq,
		(unsigned long)st.frames, (unsigned long)st.hits, st.score,
		st.box[0], st.box[1], st.box[2], st.box[3],
		(unsigned long)(st.t_infer / 1000), (unsigned long)((st.t_grab + st.t_pre) / 1000), st.error);
	return n < 0 ? 0 : (size_t)n < len ? (size_t)n : len - 1;
}

/* Run the host's golden input and compare the raw int8 outputs with what
 * LiteRT produced on the host. Blocks the console for one inference. */
static int run_golden(void)
{
	const uint8_t *g = partition_base ? partition_base + GOLDEN_OFFSET : nullptr;
	if (!g || memcmp(g, "GOLD", 4) != 0) {
		printf("no golden vectors (make -C model flash-golden)\n");
		return 1;
	}
	uint32_t len[3];
	memcpy(len, g + 4, sizeof(len));
	const int8_t *in = (const int8_t *)(g + 16);
	const int8_t *want_boxes = in + len[0], *want_scores = want_boxes + len[1];
	if (len[0] != t_in->bytes || len[1] != t_boxes->bytes || len[2] != t_scores->bytes) {
		printf("golden vectors don't match this model's tensors (%lu/%lu/%lu vs %u/%u/%u) -- same RES?\n",
		       (unsigned long)len[0], (unsigned long)len[1], (unsigned long)len[2],
		       (unsigned)t_in->bytes, (unsigned)t_boxes->bytes, (unsigned)t_scores->bytes);
		return 1;
	}
	printf("running golden input (%lu bytes), one inference...\n", (unsigned long)len[0]);
	xSemaphoreTake(infer_lock, portMAX_DELAY);
	memcpy(t_in->data.int8, in, len[0]);
	memcpy(pending_in, in, len[0]);
	profiler.Reset();
	pie_elementwise_reset_stats();
	pie_parallel_reset_stats();
	int64_t t0 = esp_timer_get_time();
	TfLiteStatus rc = interpreter->Invoke();
	int64_t dt = esp_timer_get_time() - t0;
	int worst = 0, diff_b = 0, diff_s = 0;
	for (uint32_t i = 0; i < len[1]; i++) {
		int d = abs(t_boxes->data.int8[i] - want_boxes[i]);
		if (d) diff_b++;
		if (d > worst) worst = d;
	}
	for (uint32_t i = 0; i < len[2]; i++) {
		int d = abs(t_scores->data.int8[i] - want_scores[i]);
		if (d) diff_s++;
		if (d > worst) worst = d;
	}
	int best = 0;
	for (int i = 0; i < n_anchors; i++)
		if (t_scores->data.int8[i] > t_scores->data.int8[best]) best = i;
	float score = sigmoidf(dequant(t_scores, t_scores->data.int8[best]));
	/* model-input coordinates: identity letterbox */
	Letterbox ident = { 1.0f, 0, 0 };
	publish_snapshot(ident, res, res, dt / 1000, 1);
	xSemaphoreGive(infer_lock);
	if (rc != kTfLiteOk) {
		printf("Invoke failed\n");
		return 1;
	}
	printf("inference %.1f ms; best score %.3f (anchor %d)\n", dt / 1e3, score, best);
	printf("vs host LiteRT (reference kernels): boxes %d/%lu differ, scores %d/%lu differ, max |diff| %d LSB -> %s\n",
	       diff_b, (unsigned long)len[1], diff_s, (unsigned long)len[2], worst,
	       worst == 0 ? "BIT-EXACT" : worst <= 1 ? "within 1 LSB" : "MISMATCH");
	return worst > 1;
}

static int cmd_det(int argc, char **argv)
{
	const char *sub = argc > 1 ? argv[1] : "";
	if (strcmp(sub, "on") == 0 || strcmp(sub, "off") == 0) {
		if (!interpreter) {
			printf("no model: %s\n", st.error);
			return 1;
		}
		enabled = sub[1] == 'n';
		return 0;
	}
	if (strcmp(sub, "thr") == 0 && argc > 2) {
		float t = atof(argv[2]);
		if (t <= 0 || t >= 1) {
			printf("threshold must be in (0, 1)\n");
			return 1;
		}
		threshold = t;
		return 0;
	}
	if (strcmp(sub, "pie") == 0) {
		const char *a = argc > 2 ? argv[2] : "stats";
		if (strcmp(a, "on") == 0 || strcmp(a, "off") == 0) pie_set_enabled(a[1] == 'n');
		else if (strcmp(a, "verify") == 0) pie_set_verify(argc > 3 && strcmp(argv[3], "on") == 0);
		else if (strcmp(a, "bench") == 0) pie_microbench();
		else if (strcmp(a, "cores") == 0 && argc > 3) pie_parallel_set_cores(atoi(argv[3]));
		else if (strcmp(a, "fuse") == 0 && argc > 3) pie_fusion_set_enabled(strcmp(argv[3], "on") == 0);
		else pie_print_stats();
		printf("PIE kernels %s, %d core%s\n", pie_enabled() ? "on" : "off", pie_parallel_cores(), pie_parallel_cores() > 1 ? "s" : "");
		return 0;
	}
	if (strcmp(sub, "golden") == 0) {
		if (!interpreter) {
			printf("no model: %s\n", st.error);
			return 1;
		}
		return run_golden();
	}
	if (strcmp(sub, "prof") == 0) {
		/* snapshot, sort by time */
		OpProfiler::Row rows[OpProfiler::kMaxTags];
		int n = profiler.n_rows;
		memcpy(rows, profiler.rows, n * sizeof(rows[0]));
		int64_t total = 0;
		for (int i = 0; i < n; i++) total += rows[i].us;
		for (int i = 0; i < n; i++)
			for (int j = i + 1; j < n; j++)
				if (rows[j].us > rows[i].us) { auto t = rows[i]; rows[i] = rows[j]; rows[j] = t; }
		printf("%-22s %6s %10s %6s\n", "op", "count", "ms", "%");
		for (int i = 0; i < n; i++)
			printf("%-22s %6lu %10.1f %5.1f%%\n", rows[i].tag, (unsigned long)rows[i].count,
			       rows[i].us / 1e3, total ? 100.0 * rows[i].us / total : 0);
		printf("%-22s %6s %10.1f\n", "total", "", total / 1e3);
		return 0;
	}
	if (strcmp(sub, "dump") == 0) {
		/* the BLE detsnap payload, as hex -- for when BLE is busy */
		size_t n = detector_snapshot_size();
		uint8_t *buf = n ? (uint8_t *)heap_caps_malloc(n, MALLOC_CAP_SPIRAM) : nullptr;
		if (!buf || !detector_snapshot(buf, n)) {
			printf("no snapshot yet\n");
			heap_caps_free(buf);
			return 1;
		}
		printf("-----BEGIN DETSNAP %u-----\n", (unsigned)n);
		for (size_t i = 0; i < n; i++) printf("%02x%s", buf[i], (i % 64 == 63 || i == n - 1) ? "\n" : "");
		printf("-----END DETSNAP-----\n");
		heap_caps_free(buf);
		return 0;
	}
	if (sub[0]) {
		printf("usage: det [on|off|thr <0..1>|prof|golden|dump|pie [on|off|stats|verify on|off|bench|cores 1|2|fuse on|off]]\n");
		return 1;
	}
	printf("state      %s%s%s\n", enabled ? "running" : interpreter ? "off" : "no model",
	       st.error[0] ? " -- " : "", st.error);
	if (!interpreter) return 0;
	printf("model      %dx%d int8 input, %d anchors, arena %u KB used\n",
	       res, res, n_anchors, (unsigned)(st.arena_used / 1024));
	printf("input q    scale %g zp %d\n", t_in->params.scale, (int)t_in->params.zero_point);
	printf("frames     %lu (%lu with hand), threshold %.2f\n",
	       (unsigned long)st.frames, (unsigned long)st.hits, threshold);
	printf("last       score %.3f, %d anchors above thr, box %.0f,%.0f - %.0f,%.0f\n",
	       st.score, st.above, st.box[0], st.box[1], st.box[2], st.box[3]);
	printf("timing     grab+decode %.1f  preprocess %.1f  inference %.1f  post %.1f ms\n",
	       st.t_grab / 1e3, st.t_pre / 1e3, st.t_infer / 1e3, st.t_post / 1e3);
	return 0;
}

void detector_register_cli_commands(void)
{
	const esp_console_cmd_t cmd = {
		.command = "det",
		.help = "Palm detector: status | on | off | thr <0..1> | prof (per-op timing of the last inference) | golden (compare with host LiteRT) | dump (last snapshot as hex) | pie on/off/stats/verify on/off/bench/cores 1|2/fuse on|off",
		.hint = NULL,
		.func = &cmd_det,
		.argtable = NULL,
		.func_w_context = NULL,
		.context = NULL,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&cmd));
}
