#include <stdbool.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"

#include "node_config.h"
#include "state.h"
#include "led_task.h"

void led_task_init(void)
{
	gpio_reset_pin(LED_GPIO);
	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);
}

/* Polls the counter every 100ms; toggles the LED whenever it has changed
 * since the last poll. */
void led_task(void *arg)
{
	uint32_t last_seen = state_counter_read();
	bool led_on = false;

	while (1) {
		uint32_t current = state_counter_read();

		if (current != last_seen) {
			led_on = !led_on;
			gpio_set_level(LED_GPIO, led_on);
			last_seen = current;
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}
