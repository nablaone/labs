#include <stdbool.h>
#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"

#include "node_config.h"
#include "state.h"
#include "heartbeat_task.h"

#define HB_MS_PER_TICK 100 /* 100ms per tick, so a period of 5 ticks = 500ms */

void heartbeat_task_init(void)
{
	gpio_reset_pin(LED_GPIO);
	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);
}

/* Ticks every HB_MS_PER_TICK; increments the counter once every 5 ticks
 * (500ms) and toggles the LED whenever that read-back shows the counter
 * actually changed -- which also catches a change from elsewhere (e.g.
 * CAN RX bumping it in can_rx_task), not just this task's own tick. */
void heartbeat_task(void *arg)
{
	bool led_on = false;
	uint32_t ticks = 0;
	uint32_t previous_counter = state_counter_read();

	while (1) {
		vTaskDelay(pdMS_TO_TICKS(HB_MS_PER_TICK));

		if (++ticks == 5) {
			state_counter_increment();
			ticks = 0;
		}

		uint32_t current_counter = state_counter_read();
		if (current_counter != previous_counter) {
			previous_counter = current_counter;
			led_on = !led_on;
			gpio_set_level(LED_GPIO, led_on);
		}
	}
}

