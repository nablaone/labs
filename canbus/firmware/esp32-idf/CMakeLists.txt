cmake_minimum_required(VERSION 3.16)

# By default idf.py compiles every component ESP-IDF ships (wifi_provisioning,
# mqtt, spiffs, fatfs, json, protocomm, espcoredump, etc.), not just what this
# app actually requires -- it only trims the final link, not the build. None
# of that ends up in the binary (see `idf.py size-components`), but it costs
# real build time on every clean build. This app is USB-serial CLI + GPIO +
# I2C + CAN (TWAI) only, so restrict the component search to `main` --
# project.cmake still resolves and includes main's own (PRIV_)REQUIRES chain
# plus the baseline components every ESP-IDF app needs (freertos, esp_system,
# bootloader, partition_table, etc.) automatically.
set(COMPONENTS main)

include($ENV{IDF_PATH}/tools/cmake/project.cmake)
project(canbus-esp32-idf)
