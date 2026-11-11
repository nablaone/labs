# handdetector

ESP32-S3/ESP-IDF firmware for hand detection with a camera. Forked from
`../dcc` with everything DCC/BLE stripped, so only the console/CLI, the
heartbeat task and the `app_main` skeleton came over; camera support was
added on top.

- Board: **DFRobot FireBeetle 2 ESP32-S3-U (DFR0975-U)**, an
  ESP32-S3-WROOM-1U-N16R8 (`esptool chip-id`: ESP32-S3 QFN56 rev v0.1,
  8 MB octal PSRAM, 16 MB quad flash). Console over native
  USB-Serial/JTAG at `/dev/cu.usbmodem*`. LED GPIO21 active-high.
- Camera: **OV2640** on the board's DVP connector. Its supplies come from
  an **AXP313A** PMIC (I2C 0x36) that must be switched on first. The PMIC
  and the camera's SCCB share I2C on GPIO1/2: `pmic.c` opens the bus once
  and esp32-camera borrows it (`pin_sccb_sda = -1`), which needs ESP-IDF
  >= 5.4 (older IDF makes esp32-camera use the legacy I2C driver, which
  aborts when mixed with `i2c_master`).
- Images leave the board over BLE (`main/ble_link.c`, protocol in
  `ble_link.h`) to `web/index.html` (Web Bluetooth, `make web`). Add new
  diagnostics as text commands there. All camera access goes through
  `camera_grab()`/`camera_release()` (mutex) so mode changes are safe.
- Palm detector (`main/detector.cc`): OpenCV-zoo ONNX model converted by
  `model/` (all Python/TF in the `handdetector-model` container, never on
  the host) and run with upstream TFLM reference kernels -- the user
  explicitly does not want Espressif ML libraries (ESP-NN,
  esp-tflite-micro, ESP-DL). The .tflite lives in the `model` partition,
  not in the firmware. `make -C model tflm` must run before the first
  firmware build. Findings and numbers: `docs/model-pipeline.md`.
- Build in Docker (`make build`). Flash and monitor natively (`make flash`,
  `make monitor`).
- One `.c/.h` per module: `xxx_init()`, `xxx_task()`,
  `xxx_register_cli_commands()`. `main.c` only orchestrates.
