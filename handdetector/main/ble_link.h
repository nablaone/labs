#pragma once

/*
 * BLE diagnostic link to the web page (../web/index.html, Chrome Web
 * Bluetooth). The USB console is text-only, so camera frames come out
 * this way instead.
 *
 * GATT layout (NimBLE peripheral, advertised as "HandDetector"):
 *   service  48440001-6b2e-4c8a-9f3e-5a1b2c3d4e5f
 *     ctrl   48440002-...  write          text command, see below
 *     event  48440003-...  notify         text reply per command ("ok ..." / "err ...")
 *     info   48440004-...  read           JSON status (camera, firmware, heap, link)
 *     frame  48440005-...  notify         image stream, see below
 *
 * Commands (ASCII, one per write):
 *   snap                 send one frame
 *   stream on|off        send frames continuously
 *   fmt jpeg|gray|rgb565 pixel format (camera re-init)
 *   res <size>           96x96 qqvga qcif 240x240 qvga cif vga svga xga sxga uxga
 *   q <4..63>            JPEG quality (lower = better)
 *   xclk <5..24>         sensor clock in MHz (camera re-init)
 *   set <param> <value>  live sensor control (colorbar, agc, gain, aec,
 *                        exposure, gainceiling, ... see camera.h)
 *   reg <0xBAA> [value]  raw OV2640 register read/write, B = bank
 *                        (0 DSP, 1 sensor); replies "ok reg 0xBAA 0xVV"
 *   info                 just reply "ok" (the page re-reads info after every reply)
 *
 * Frame notifications, all little-endian. Byte 0 is the record type:
 *   0x01 header  u8 type, u16 frame_id, u32 total_len, u16 width,
 *                u16 height, u8 pixformat (esp32-camera pixformat_t:
 *                0 RGB565 big-endian, 3 GRAYSCALE, 4 JPEG),
 *                u32 timestamp_ms, u32 crc32 (zlib/IEEE of the payload),
 *                u16 reserved (0)   -- 22 bytes
 *   0x02 data    u8 type, u16 frame_id, u32 offset, payload bytes...
 * A frame is complete when offset + len reaches total_len. Chunks are
 * sized to the negotiated MTU.
 */

/* NVS + NimBLE host + GATT service + advertising. */
void ble_link_init(void);

/* Runs commands and pushes frames while a client is subscribed. */
void ble_link_task(void *arg);

/* Registers the "ble" CLI command (connection state and counters). */
void ble_link_register_cli_commands(void);
