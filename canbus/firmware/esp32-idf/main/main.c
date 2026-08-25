#include <stdbool.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"
#include "esp_log.h"

/*
 * LED on GPIO2 (onboard LED on this DevKitC-compatible board) and button on
 * GPIO33, matching the wiring worked out for the earlier Zephyr app -- see
 * ../../CLAUDE.md and boards/esp32_devkitc_esp32_procpu.overlay in
 * ../zephyr-canbus for the reasoning (GPIO2 is a strapping pin but its
 * onboard-LED loading doesn't disturb boot-mode sensing in practice;
 * GPIO33 avoids strapping pins and the default TWAI RX/TX pins for later
 * CAN work).
 */
#define LED_GPIO    GPIO_NUM_2
#define BUTTON_GPIO GPIO_NUM_33

#define SLOW_BLINK_MS 1000
#define FAST_BLINK_MS 100

static const char *TAG = "canbus";

void app_main(void)
{
	gpio_reset_pin(LED_GPIO);
	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);

	gpio_reset_pin(BUTTON_GPIO);
	gpio_set_direction(BUTTON_GPIO, GPIO_MODE_INPUT);
	gpio_pullup_en(BUTTON_GPIO);
	gpio_pulldown_dis(BUTTON_GPIO);

	bool led_on = false;

	while (1) {
		/* active-low: button pressed means the pin reads 0 */
		bool pressed = gpio_get_level(BUTTON_GPIO) == 0;
		int period_ms = pressed ? FAST_BLINK_MS : SLOW_BLINK_MS;

		led_on = !led_on;
		gpio_set_level(LED_GPIO, led_on);
		ESP_LOGI(TAG, "led %s (period=%dms)", led_on ? "on" : "off", period_ms);

		vTaskDelay(pdMS_TO_TICKS(period_ms));
	}
}
