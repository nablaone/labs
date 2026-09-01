idf_component_register(
    SRCS "main.c" "state.c" "led_task.c" "heartbeat_task.c" "button_task.c"
         "display_task.c" "can.c" "console.c" "lcd_task.c"
    INCLUDE_DIRS "."
    PRIV_REQUIRES console esp_driver_gpio esp_driver_uart esp_driver_i2c driver vfs esp_vfs_console)
