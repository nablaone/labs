# dcc

Educational ESP32/ESP-IDF project: a DCC accessory (turnout) decoder for
a PIKO H0 layout, modelled on the PIKO #55030 switch decoder. Forked from
`../canbus/firmware/esp32-idf` with everything CAN-related stripped, so
only the console/CLI and the task/`app_main` skeleton remain.

- Board: **ESP32-C3 Super Mini** (`IDF_TARGET=esp32c3`, console over
  native USB-Serial/JTAG at `/dev/cu.usbmodem*`, LED GPIO8 active-low,
  BOOT GPIO9). Identified with `esptool flash-id`: ESP32-C3 QFN32 rev
  v0.4, 4 MB embedded flash.
- Build in Docker (`make build`). Flash and monitor natively (`make flash`,
  `make monitor`). See README.md.
- One `.c/.h` per module: `xxx_init()`, `xxx_task()`,
  `xxx_register_cli_commands()`. `main.c` only orchestrates.
- Reference docs live in `docs/`. The PDFs are gitignored (copyright),
  and their URLs are in `docs/README.md`. `docs/dcc-accessory-primer.md`
  is the hand-written summary: timing, packet format, addressing,
  hardware, and the implementation plan.
- It's a learning project, so explain the DCC/ESP-IDF reasoning in
  comments and docs rather than just writing code.
