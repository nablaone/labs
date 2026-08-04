# Zephyr dev image: toolchain + Zephyr source baked in, application source
# is bind-mounted from the host at /app (see Makefile) rather than copied
# in, so editing on the host and rebuilding in the container is instant.
#
# First build fetches the full Zephyr workspace (~1-2GB) plus the ARM and
# Xtensa SDK toolchains (~1-2GB combined) — expect it to take a while and
# produce a multi-GB image. Subsequent builds are cached unless
# ZEPHYR_REVISION changes.

FROM debian:trixie-slim

ARG ZEPHYR_REVISION=v4.4.0
ENV DEBIAN_FRONTEND=noninteractive

# Official Zephyr host dependency list (Getting Started Guide), Debian/Ubuntu
# variant, minus gcc-multilib/g++-multilib: those are only for building
# native_sim/host test binaries, not available on arm64 (no i386 multilib
# equivalent), and irrelevant for cross-compiling to Cortex-M via the SDK.
RUN apt-get update && apt-get install -y --no-install-recommends \
        git cmake ninja-build gperf ccache dfu-util device-tree-compiler wget \
        python3-dev python3-venv python3-pip python3-tk xz-utils file make \
        gcc libsdl2-dev libmagic1 \
        curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Debian marks the system Python as externally-managed; use a venv instead
# of --break-system-packages so `pip install` behaves normally.
ENV VIRTUAL_ENV=/opt/venv
RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"
RUN pip install --no-cache-dir west

# Large clones over the default HTTP/2 stack have been flaky (mid-transfer
# TLS record errors) -- HTTP/1.1 + a bigger post buffer works around it.
RUN git config --global http.postBuffer 1048576000 \
    && git config --global http.version HTTP/1.1

WORKDIR /opt/zephyrproject
# `west init`'s own manifest-repo clone isn't shallow even with --narrow (that
# only affects `west update`'s module clones) -- so shallow-clone Zephyr
# ourselves first and point west at it with `-l` instead.
# `west update` fetches dozens of module repos; on a flaky connection at
# least one of them tends to hit the same kind of transient clone failure
# as above, so retry the whole command a few times (already-fetched modules
# are left alone on a retry, only the failed one(s) redo any work).
RUN git clone --depth 1 --branch ${ZEPHYR_REVISION} https://github.com/zephyrproject-rtos/zephyr zephyr \
    && west init -l zephyr \
    && (west update --narrow -o=--depth=1 \
        || west update --narrow -o=--depth=1 \
        || west update --narrow -o=--depth=1) \
    && west zephyr-export \
    && pip install --no-cache-dir -r zephyr/scripts/requirements.txt

# Kept as its own layer (not appended above) so tweaking it doesn't bust the
# cache on the expensive west update step.
RUN pip install --no-cache-dir esptool

# ARM toolchain for the Pico (Cortex-M), Xtensa toolchain for the ESP32 --
# skips every other architecture's toolchain to save download/disk space.
# Retried like west update above -- this download has stalled mid-transfer
# on a flaky connection too.
RUN (west sdk install --toolchains arm-zephyr-eabi xtensa-espressif_esp32_zephyr-elf \
    || west sdk install --toolchains arm-zephyr-eabi xtensa-espressif_esp32_zephyr-elf \
    || west sdk install --toolchains arm-zephyr-eabi xtensa-espressif_esp32_zephyr-elf)

ENV ZEPHYR_BASE=/opt/zephyrproject/zephyr

# Espressif's QEMU fork -- real Xtensa CPU + peripheral emulation for ESP32,
# unlike native_sim (a host binary with GPIO faked). Prebuilt binary release,
# not built from source (much faster, and doesn't need the full X11/SDL/GTK
# build dependency chain). aarch64-linux-gnu matches this image's arch
# (built on Apple Silicon); switch to the x86_64-linux-gnu asset if building
# on an Intel host.
# Own apt-get layer (not folded into the main one above) so touching QEMU
# deps doesn't bust the cache on the expensive west update/sdk install
# steps that come before it.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libpixman-1-0 libslirp0 libgcrypt20 \
    && rm -rf /var/lib/apt/lists/*

ARG QEMU_ESP_RELEASE=esp-develop-9.2.2-20260417
RUN wget -qO /tmp/qemu-esp.tar.xz \
        "https://github.com/espressif/qemu/releases/download/${QEMU_ESP_RELEASE}/qemu-xtensa-softmmu-esp_develop_9.2.2_20260417-aarch64-linux-gnu.tar.xz" \
    && mkdir -p /opt/espressif-qemu \
    && tar -xf /tmp/qemu-esp.tar.xz -C /opt/espressif-qemu --strip-components=1 \
    && rm /tmp/qemu-esp.tar.xz
ENV PATH="/opt/espressif-qemu/bin:${PATH}"
RUN qemu-system-xtensa --version

CMD ["bash"]
