IMAGE ?= canbus-zephyr-dev
BOARD ?= rpi_pico
BUILD_DIR = build-$(subst /,_,$(BOARD))
APP_DIR := $(CURDIR)

.PHONY: image build run run-esp32-qemu shell flash flash-esp32 console-esp32 clean

image:
	docker build -t $(IMAGE) .

build: image
	docker run --rm -v $(APP_DIR):/app $(IMAGE) \
		west build -p auto -b $(BOARD) -s /app -d /app/$(BUILD_DIR)

# Run in Zephyr's native_sim emulator -- a host Linux binary, no real
# hardware needed. LED/button are emulated (see boards/native_sim.overlay);
# main.c auto-toggles the emulated button every 3s to demo the blink-rate
# change. Ctrl-C to exit.
run: BOARD := native_sim/native/64
run: build
	docker run --rm -it -v $(APP_DIR):/app $(IMAGE) /app/$(BUILD_DIR)/zephyr/zephyr.exe

# Run the real esp32_devkitc build in Espressif's QEMU fork -- an actual
# Xtensa CPU + peripheral emulator, unlike native_sim (a host binary with
# GPIO faked). Validates that the app boots and runs against a real(ish)
# hardware model. QEMU's keyboard-driven touch-pin button simulation
# (keys 7/8/9/0 -> GPIO01/02/12/13) doesn't reach this app's button pin
# (GPIO33, chosen for real-hardware wiring reasons) -- so this doesn't
# demo the blink-rate change interactively the way `make run` does; it's
# a boot/sanity check against closer-to-real hardware. Ctrl-A then X to
# exit.
run-esp32-qemu: BOARD := esp32_devkitc/esp32/procpu
run-esp32-qemu: build
	docker run --rm -it -v $(APP_DIR):/app -w /app/$(BUILD_DIR)/zephyr $(IMAGE) sh -c \
		"dd if=/dev/zero of=zephyr_4mb.bin bs=1M count=4 2>/dev/null && \
		 dd if=zephyr.bin of=zephyr_4mb.bin bs=1 seek=4096 conv=notrunc 2>/dev/null && \
		 qemu-system-xtensa -nographic -machine esp32 -drive file=zephyr_4mb.bin,if=mtd,format=raw"

# Needs USB access to the debug probe. Broad --privileged + /dev mount for
# now since the actual probe (picoprobe/CMSIS-DAP) isn't chosen yet --
# tighten to a specific --device once it is. Linux host only, see
# ../../docs/dev-setup-research.md for the macOS USB-passthrough caveat.
flash: image
	docker run --rm --privileged -v /dev:/dev -v $(APP_DIR):/app $(IMAGE) \
		west flash -d /app/$(BUILD_DIR)

# ESP32 flashes over plain USB-serial via esptool -- no debug probe, and
# (unlike SWD probes) works fine straight from a native esptool install on
# the Mac, no Docker/Linux-box USB-passthrough detour needed. Not run
# through the container for that reason.
# Binary name varies by install method: pip's esptool package provides both
# `esptool` and `esptool.py`, but Debian's apt package only provides
# `esptool` -- pick whichever exists.
ESPTOOL := $(shell command -v esptool.py 2>/dev/null || command -v esptool 2>/dev/null || echo esptool.py)
# --no-stub: Debian's esptool package is a +dfsg build that strips the
# prebuilt stub-flasher blobs esptool normally uploads and runs on the chip
# for faster flashing -- without them it dies with FileNotFoundError
# (confirmed 2026-08-05 on this host's esptool 4.7.0+dfsg-0.1). --no-stub
# talks directly to the ROM bootloader instead, slower but works
# everywhere regardless of install method.
# make flash-esp32 PORT=/dev/tty.usbserial-XXXX
flash-esp32: BOARD := esp32_devkitc/esp32/procpu
flash-esp32:
	$(ESPTOOL) --chip esp32 --no-stub -p $(PORT) write_flash 0x1000 $(BUILD_DIR)/zephyr/zephyr.bin

# Watch the ESP32's serial console (115200 8N1, Zephyr's console default --
# confirmed 2026-08-05 against real hardware). Native on the host like
# flash-esp32, same macOS/Linux-passthrough reasoning. Ctrl-A X to exit.
# make console-esp32 PORT=/dev/tty.usbserial-XXXX
console-esp32:
	minicom -D $(PORT) -b 115200

shell: image
	docker run --rm -it -v $(APP_DIR):/app $(IMAGE) bash

clean:
	rm -rf build-*
