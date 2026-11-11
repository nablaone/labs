#pragma once

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Palm detector: OpenCV-zoo MediaPipe palm detection, converted by the
 * ../model pipeline (ONNX -> surgery -> int8 TFLite) and run with upstream
 * TensorFlow Lite Micro (reference kernels).
 *
 * The .tflite lives in its own flash partition ("model", see
 * ../partitions.csv; `make -C model flash-model`) and is memory-mapped,
 * not copied. Input size comes from the model's input tensor, so models
 * built with a different RES just work; anchors are generated to match.
 *
 * Each frame: grab (JPEG or RGB565) -> RGB -> letterbox to the model input
 * -> int8 -> Invoke -> best anchor -> sigmoid -> threshold. A hit makes the
 * heartbeat LED blink fast.
 */

/* Maps the model partition, sets up the interpreter. Logs and leaves the
 * detector disabled if no valid model is flashed. */
void detector_init(void);

/* Grab -> infer -> LED, forever. Pin to core 1: an inference takes
 * seconds and core 0 runs BLE + the console. */
void detector_task(void *arg);

/* "det" CLI command. */
void detector_register_cli_commands(void);

/* Number of the last inference (0 = none yet; also bumped by `det golden`)
 * plus its best score and whether that crossed the threshold. */
uint32_t detector_last(float *score, bool *hand);

/* Last result as a compact JSON object, for the BLE info characteristic. */
size_t detector_info_json(char *buf, size_t len);

/*
 * Snapshot of the last inference for the web page's detection view: what
 * the model saw and everything it output. Sent over BLE as a "frame" with
 * pixformat DET_SNAPSHOT_FORMAT. Little-endian, 60-byte header:
 *
 *   0  "DET1"            4  u16 res          6  u16 n_anchors
 *   8  u16 src_w        10  u16 src_h       12  f32 letterbox ratio
 *  16  i16 lb_left      18  i16 lb_top      20  f32 input scale
 *  24  i32 input zp     28  f32 box scale   32  i32 box zp
 *  36  f32 score scale  40  i32 score zp    44  f32 threshold
 *  48  u32 seq          52  u32 infer_ms    56  u8 source (0 camera,
 *                                               1 golden), 3 pad
 *  60  int8 input[res*res*3] (NHWC RGB), int8 scores[n_anchors],
 *      int8 boxes[n_anchors*4] (dcx, dcy, w, h)
 *
 * Anchor centres aren't sent; the page generates them like the firmware
 * (strides 8/16, 2/6 anchors per cell).
 */
#define DET_SNAPSHOT_FORMAT 0x80
size_t detector_snapshot_size(void);   /* 0 if no inference yet */
size_t detector_snapshot(uint8_t *buf, size_t cap);

#ifdef __cplusplus
}
#endif
