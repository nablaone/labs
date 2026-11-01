# ESP-IDF dev image: the official Espressif image already bakes in the
# toolchain, idf.py, and a Python venv with everything activated via its
# entrypoint (sources $IDF_PATH/export.sh before running the given command)
# -- unlike the Zephyr setup, there's no workspace to fetch or SDK to
# install ourselves. Application source is bind-mounted from the host at
# /app (see Makefile) rather than copied in, so editing on the host and
# rebuilding in the container is instant.

ARG IDF_VERSION=v5.3.2
FROM espressif/idf:${IDF_VERSION}

WORKDIR /app
