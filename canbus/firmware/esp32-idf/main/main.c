#include <stdbool.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"
#include "esp_log.h"

/*
 * LED on GPIO2 (onboard LED) and button on GPIO0 (onboard BOOT button) --
 * both onboard, no breadboard wiring needed. Both are strapping pins
 * (sampled at boot to select flash/boot mode), which is why the earlier
 * Zephyr app avoided them in favor of an external button on GPIO33 -- but
 * once the app is running, GPIO0 reads like any other input (it's only
 * sampled at reset), and the onboard LED's light loading on GPIO2 doesn't
 * disturb boot-mode sensing in practice (confirmed on real hardware). The
 * board already has an external pull-up on GPIO0 for its BOOT button;
 * gpio_pullup_en() below just reinforces it.
 */
#define LED_GPIO    GPIO_NUM_2
#define BUTTON_GPIO GPIO_NUM_0

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
