idf_component_register(
    SRCS "main.c" "state.c" "heartbeat_task.c" "metrics.c"
         "display_task.c" "can.c" "console.c" "identity.c"
         "ping_role.c" "pong_role.c" "pot_sender_role.c" "pot_collector_role.c"
    INCLUDE_DIRS "."
    PRIV_REQUIRES console esp_driver_gpio esp_driver_uart esp_driver_i2c driver vfs esp_vfs_console nvs_flash esp_timer esp_adc)
