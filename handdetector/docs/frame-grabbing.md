# Frame grabbing: timing and image quality

Notes from measuring the OV2640 on the FireBeetle 2 ESP32-S3 on
2026-11-11 (ESP-IDF v5.4.2, `espressif/esp32-camera` 2.1.x). Camera
setup is in `main/camera.c`.

## How capture works here

- The OV2640 sends pixels over the 8-bit DVP bus. The S3's LCD_CAM
  peripheral DMAs them into **2 frame buffers in PSRAM**
  (`fb_count = 2`, `CAMERA_FB_IN_PSRAM`).
- `CAMERA_GRAB_LATEST`: old frames are dropped, so a grab always returns
  the newest one.
- Capturing uses **no CPU time**. DMA runs in the background.
  `esp_camera_fb_get()` just hands over a finished buffer, or blocks
  until the next one is finished.
- All access goes through `camera_grab()` / `camera_release()`, which
  hold a mutex so mode changes (`fmt`, `res`, `xclk`) can't free buffers
  under a reader.

So "how long does a grab take" means two different things:

- **Back to back** (grab, release, grab…): the call waits for the
  sensor's next frame, so it takes one **frame period**.
- **After slow processing** (longer than a frame period): a frame is
  already waiting, and the call returns in **~0 ms**. That's why
  `cam snap` reported 0 ms.

## Measured frame periods

`cam bench [n]` grabs n frames back to back. It skips the first 2,
which were already buffered. It reports the `fb_get` latency and the
interval between the frames' own timestamps. XCLK 20 MHz, AGC gain
capped at 2× (see below), indoor light.

| mode | bytes/frame | frame period | fps |
|---|---|---|---|
| gray 96×96 | 9 216 | 40.0 ms | 25.0 |
| gray 160×120 (QQVGA) | 19 200 | 40.0 ms | 25.0 |
| gray 320×240 (QVGA) | 76 800 | 40–43 ms (max 120) | 23.4 |
| gray 640×480 (VGA) | 307 200 | 160–480 ms | ~5.5 |
| RGB565 160×120 | 38 400 | 43.9 ms | 22.8 |
| RGB565 320×240 | 153 600 | 44.6 ms | 22.4 |
| JPEG 320×240 | ~12 KB | 20.0 ms | 50.0 |
| JPEG 640×480 | ~30 KB | 40.0 ms | 25.0 |
| JPEG 1600×1200 (UXGA) | ~200 KB | 80.0 ms | 12.5 |
| JPEG 160×120 | — | **fails** | — |

Observations:

- **Raw modes top out at 25 fps.** Going below 320×240 doesn't make
  them faster: the sensor reads the same window and its DSP scales it
  down. So 96×96 costs as much time as 320×240.
- **JPEG is fastest**, because the sensor compresses on-chip and the
  DVP bus moves far fewer bytes. To use JPEG for detection you'd have to
  decode it on the S3 first, which costs CPU.
- **Gray VGA collapses** to ~5.5 fps, with `cam_hal: EV-VSYNC-OVF`.
  That's 300 KB per frame, more than the DVP/DMA path keeps up with.
  Stay at QVGA or smaller for raw frames.
- **JPEG QQVGA fails** with `NO-EOI - JPEG end marker missing`.
  esp32-camera sizes the JPEG buffer from the resolution (about w×h/5:
  3 840 bytes at QQVGA). Noisy frames compress badly and overflow it.
  `cam_hal: FB-OVF` is the same overflow at other sizes, seen at QVGA
  when the gain is high.

**For hand detection:** grayscale (or RGB565) at 96×96–320×240 gives a
new frame every **40–44 ms (22–25 fps)**. That's the ceiling for the
whole pipeline. Inference time comes on top, unless it runs while the
next frame is being captured, which the double buffer allows.

Not measured yet:

- XCLK 24 MHz. It may raise the raw-mode ceiling to about 30 fps
  (`cam xclk 24`, then `cam bench`).
- The effect of auto exposure in dim light. With the gain capped,
  exposure can grow longer than the frame time, and the frame rate
  would then drop.

## Image noise (horizontal lines)

The sensor shows horizontal row noise that grows with analog gain.
Narrowed down on the board:

| test | result |
|---|---|
| `cam set colorbar 1` (test pattern made inside the sensor) | clean |
| AGC off, `cam set gain 0` | nearly clean |
| AGC off, `cam set gain 30` | streaks over the whole frame |
| no BLE client, frame pulled over USB (`cam dump`) | same streaks, so not radio interference |
| XCLK 10 vs 20 MHz | no difference |
| `pmic dldo 3000` (AVDD 2.8 → 3.0 V), `pmic aldo 1300` (DVDD 1.2 → 1.3 V) | no visible difference |
| `pmic` register dump | outputs 0x19, ALDO1 1.2 V, DLDO1 2.8 V, as configured |

JPEG frames decode cleanly and the colour bars are clean, so the DVP
data lines are fine. This is analog noise from the pixel array, and
gain multiplies it. Left alone, AGC raises the gain a lot indoors.

**Mitigation in firmware:** after every camera init, AGC is capped at
2× (`CAM_DEFAULT_GAINCEILING` in `camera.c`), and AEC lengthens the
exposure instead. That gives a usable image with faint lines left. In
dim light, raise the cap with `cam set gainceiling <0..6>` (2×…128×)
and accept more noise, or add light.

Worth trying on the hardware side: reseat the camera ribbon, or swap in
another OV2640 module.

## Tools

| USB CLI | BLE / web page | what |
|---|---|---|
| `cam bench [n]` | — | frame period / fps / fb_get latency |
| `cam snap` | `snap` | one frame, size and grab time |
| `cam dump` | — | frame as base64 over USB, no BLE needed |
| `cam fmt` / `res` / `q` / `xclk` | same | mode changes |
| `cam set <param> <v>` | `set <param> <v>` | live sensor control (gain, exposure, gainceiling, colorbar…) |
| `cam reg <0xBAA> [v]` | `reg <0xBAA> [v]` | raw OV2640 registers (bank 0 DSP, 1 sensor) |
| `pmic`, `pmic aldo\|dldo <mV>` | — | AXP313A dump, camera rail voltages |
