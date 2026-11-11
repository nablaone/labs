# handdetector

ESP32-S3 firmware for hand detection with a camera, in plain ESP-IDF
(FreeRTOS). Right now it's the skeleton: it boots, brings up a serial
CLI over USB, powers the camera and detects it. Hand detection is the
next step.

Started as a stripped copy of [`../dcc`](../dcc). The console, heartbeat
task and Docker build came over; the DCC decoder and BLE link did not.

## Hardware

**DFRobot FireBeetle 2 ESP32-S3-U** (DFR0975-U, external-antenna variant)
with the bundled **OV2640** camera.

| | |
|---|---|
| SoC / module | ESP32-S3-WROOM-1U-N16R8: dual-core 240 MHz, 16 MB flash, 8 MB octal PSRAM |
| USB | native USB-Serial/JTAG on the USB-C port (no bridge chip) |
| LED | GPIO21, active-high |
| PMIC | AXP313A at I2C 0x36. Powers the camera (ALDO1 1.2 V DVDD, DLDO1 2.8 V AVDD/DOVDD) |
| I2C / SCCB | SDA GPIO1, SCL GPIO2. Shared by PMIC and camera |

OV2640 DVP pins (from the DFRobot wiki's CAM table):

| XCLK | PCLK | VSYNC | HREF | Y2 | Y3 | Y4 | Y5 | Y6 | Y7 | Y8 | Y9 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 45 | 5 | 6 | 42 | 39 | 40 | 41 | 4 | 7 | 8 | 46 | 48 |

PWDN is tied low and RESET is pulled up on the board, so neither uses a GPIO.

The camera stays dark until the AXP313A turns on its LDOs, and SCCB
detection then fails. That's why `pmic_camera_power_on()` runs before
`esp_camera_init()`. The register sequence comes from DFRobot's
`DFRobot_AXP313A` Arduino library.

## Build / flash / monitor

Builds run in Docker (`espressif/idf:v5.4.2`, see `Dockerfile`). Flashing
and the monitor run natively over USB-C (`/dev/cu.usbmodem*`).

```sh
make build              # docker build + idf.py build
make flash              # esptool over USB-serial (PORT auto-detected)
make monitor            # minicom, Ctrl-A X to quit
make shell              # interactive shell in the IDF container
```

Use `PORT=/dev/cu.usbmodemXXXX` to pick the port yourself. If flashing
can't connect, hold BOOT (GPIO0) while pressing RST.

## CLI

Logs flow normally. Press **Enter** to get the `hand>` prompt (logs are
silenced while you're in it), and type `exit` to leave.

| command | |
|---|---|
| `help`, `version`, `exit` | built-ins |
| `cam` / `cam info` | sensor name, PID/VER/MID, SCCB address, current mode |
| `cam snap` | grab one frame and print its size and capture time |
| `cam init` / `cam deinit` | retry detection / release the camera |
| `i2cscan` | list devices on the GPIO1/2 bus (expect 0x30 OV2640, 0x36 AXP313A) |

## Layout

```
main/main.c            orchestration only
main/console.c         USB-Serial/JTAG + esp_console/linenoise CLI
main/heartbeat_task.c  LED blink
main/pmic.c            shared I2C bus, AXP313A camera power, i2cscan
main/camera.c          esp32-camera config + detection, `cam` command
main/node_config.h     board pin map
```
