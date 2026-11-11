idf_component_register(
    SRCS "main.c" "heartbeat_task.c" "console.c" "pmic.c" "camera.c"
    INCLUDE_DIRS "."
    PRIV_REQUIRES console esp_driver_gpio esp_driver_usb_serial_jtag esp_driver_i2c
                  vfs esp_vfs_console esp_timer esp_psram spi_flash)
