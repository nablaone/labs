#include <stdbool.h>
#include <stdint.h>
#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/semphr.h"
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

#define POLL_MS      100
#define HEARTBEAT_MS 1000
#define DISPLAY_MS   10000

static const char *TAG = "canbus";

/*
 * Shared "excitement" counter -- a stand-in for later CAN-driven events
 * (each interesting bus event will eventually bump this). Three tasks
 * touch it concurrently, possibly on either of the ESP32's two cores, so
 * it's protected by a mutex rather than relying on plain uint32_t
 * reads/writes being atomic.
 */
static uint32_t excitement_counter;
static SemaphoreHandle_t counter_mutex;

static uint32_t counter_read(void)
{
	xSemaphoreTake(counter_mutex, portMAX_DELAY);
	uint32_t value = excitement_counter;
	xSemaphoreGive(counter_mutex);
	return value;
}

static void counter_increment(void)
{
	xSemaphoreTake(counter_mutex, portMAX_DELAY);
	excitement_counter++;
	xSemaphoreGive(counter_mutex);
}

/* Polls the counter every 100ms; toggles the LED whenever it has changed
 * since the last poll. */
static void led_task(void *arg)
{
	uint32_t last_seen = counter_read();
	bool led_on = false;

	while (1) {
		uint32_t current = counter_read();

		if (current != last_seen) {
			led_on = !led_on;
			gpio_set_level(LED_GPIO, led_on);
			ESP_LOGI(TAG, "led %s (counter=%" PRIu32 ")", led_on ? "on" : "off", current);
			last_seen = current;
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}

/* Increments the counter once a second on its own -- a steady background
 * source of "excitement" independent of the button. */
static void heartbeat_task(void *arg)
{
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(HEARTBEAT_MS));
		counter_increment();
	}
}

/* Polls the button every 100ms; increments the counter on every poll where
 * it reads pressed, so holding it down keeps incrementing rather than
 * counting only the initial press. */
static void button_task(void *arg)
{
	while (1) {
		/* active-low: button pressed means the pin reads 0 */
		if (gpio_get_level(BUTTON_GPIO) == 0) {
			counter_increment();
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}

void app_main(void)
{
	gpio_reset_pin(LED_GPIO);
	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);

	gpio_reset_pin(BUTTON_GPIO);
	gpio_set_direction(BUTTON_GPIO, GPIO_MODE_INPUT);
	gpio_pullup_en(BUTTON_GPIO);
	gpio_pulldown_dis(BUTTON_GPIO);

	counter_mutex = xSemaphoreCreateMutex();

	xTaskCreate(led_task, "led", 3072, NULL, 5, NULL);
	xTaskCreate(heartbeat_task, "heartbeat", 3072, NULL, 5, NULL);
	xTaskCreate(button_task, "button", 3072, NULL, 5, NULL);

	/* Main task: just displays the counter every 10s. */
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(DISPLAY_MS));
		ESP_LOGI(TAG, "excitement counter = %" PRIu32, counter_read());
	}
}
