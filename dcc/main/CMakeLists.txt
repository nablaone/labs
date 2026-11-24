idf_component_register(
    SRCS "main.c" "heartbeat_task.c" "console.c" "dcc.c" "ble_link.c" "leds.c"
    INCLUDE_DIRS "."
    PRIV_REQUIRES console esp_driver_gpio esp_driver_usb_serial_jtag vfs esp_vfs_console esp_timer
                  esp_driver_rmt
                  bt nvs_flash)
