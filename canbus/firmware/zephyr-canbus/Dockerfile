# Zephyr dev image: toolchain + Zephyr source baked in, application source
# is bind-mounted from the host at /app (see Makefile) rather than copied
# in, so editing on the host and rebuilding in the container is instant.
#
# First build fetches the full Zephyr workspace (~1-2GB) plus the ARM SDK
# toolchain (~1-2GB) — expect it to take a while and produce a multi-GB
# image. Subsequent builds are cached unless ZEPHYR_REVISION changes.

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
RUN git clone --depth 1 --branch ${ZEPHYR_REVISION} https://github.com/zephyrproject-rtos/zephyr zephyr \
    && west init -l zephyr \
    && west update --narrow -o=--depth=1 \
    && west zephyr-export \
    && pip install --no-cache-dir -r zephyr/scripts/requirements.txt

# ARM toolchain only (this lab targets Cortex-M on Pico + STM32) — skips
# the other architectures' toolchains to save a lot of download/disk space.
RUN west sdk install --toolchains arm-zephyr-eabi

ENV ZEPHYR_BASE=/opt/zephyrproject/zephyr

CMD ["bash"]
