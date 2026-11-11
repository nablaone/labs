idf_component_register(
    SRCS "main.c" "heartbeat_task.c" "console.c" "pmic.c" "camera.c" "ble_link.c" "detector.cc" "pie_conv.cc" "pie_elementwise.cc"
    INCLUDE_DIRS "."
    PRIV_REQUIRES console esp_driver_gpio esp_driver_usb_serial_jtag esp_driver_i2c
                  vfs esp_vfs_console esp_timer esp_psram spi_flash
                  bt nvs_flash esp_rom esp_partition tflite-micro)

# The detector's per-pixel preprocessing loop; the rest of the app keeps
# the project-wide (debug-friendly) optimisation level.
set_source_files_properties(detector.cc pie_conv.cc pie_elementwise.cc PROPERTIES COMPILE_OPTIONS "-O2")
