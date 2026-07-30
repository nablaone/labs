IMAGE ?= canbus-zephyr-dev
BOARD ?= rpi_pico
BUILD_DIR = build-$(subst /,_,$(BOARD))
APP_DIR := $(CURDIR)

.PHONY: image build run shell flash clean

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

# Needs USB access to the debug probe. Broad --privileged + /dev mount for
# now since the actual probe (picoprobe/CMSIS-DAP/ST-Link) isn't chosen
# yet -- tighten to a specific --device once it is. Linux host only, see
# ../../docs/dev-setup-research.md for the macOS USB-passthrough caveat.
flash: image
	docker run --rm --privileged -v /dev:/dev -v $(APP_DIR):/app $(IMAGE) \
		west flash -d /app/$(BUILD_DIR)

shell: image
	docker run --rm -it -v $(APP_DIR):/app $(IMAGE) bash

clean:
	rm -rf build-*
