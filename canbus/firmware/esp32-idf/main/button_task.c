#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"

#include "node_config.h"
#include "state.h"
#include "button_task.h"

void button_task_init(void)
{
	gpio_reset_pin(BUTTON_GPIO);
	gpio_set_direction(BUTTON_GPIO, GPIO_MODE_INPUT);
	gpio_pullup_en(BUTTON_GPIO);
	gpio_pulldown_dis(BUTTON_GPIO);
}

/* Polls the button every 100ms; increments the counter on every poll where
 * it reads pressed, so holding it down keeps incrementing rather than
 * counting only the initial press. */
void button_task(void *arg)
{
	while (1) {
		/* active-low: button pressed means the pin reads 0 */
		if (gpio_get_level(BUTTON_GPIO) == 0) {
			state_counter_increment();
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}
