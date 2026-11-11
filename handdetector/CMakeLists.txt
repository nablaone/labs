cmake_minimum_required(VERSION 3.16)

# By default idf.py compiles every component ESP-IDF ships (wifi_provisioning,
# mqtt, spiffs, fatfs, json, protocomm, espcoredump, etc.), not just what this
# app actually requires -- it only trims the final link, not the build.
# Restricting the component search to `main` keeps clean builds fast;
# project.cmake still resolves main's own (PRIV_)REQUIRES chain plus the
# baseline components every ESP-IDF app needs (freertos, esp_system,
# bootloader, partition_table, etc.) automatically.
set(COMPONENTS main)

include($ENV{IDF_PATH}/tools/cmake/project.cmake)
project(handdetector)
