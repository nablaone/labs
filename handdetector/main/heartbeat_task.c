#include <stdbool.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"

#include "node_config.h"
#include "heartbeat_task.h"

#define HB_PERIOD_MS     1000  /* alive: 1 s on, 1 s off */
#define HB_FAST_PERIOD_MS  40  /* hand detected: ~12 blinks/s */

static volatile bool fast;
static TaskHandle_t hb_task;

void heartbeat_set_fast(bool on)
{
	bool changed = fast != on;
	fast = on;
	/* switch now, not after the current (up to 1 s) wait */
	if (changed && hb_task) xTaskNotifyGive(hb_task);
}

static void led_set(bool on)
{
	gpio_set_level(LED_GPIO, on ? LED_ON_LEVEL : !LED_ON_LEVEL);
}

void heartbeat_task_init(void)
{
	gpio_reset_pin(LED_GPIO);
	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);
	led_set(false);
}

/* Toggles the LED every HB_PERIOD_MS -- a visible "firmware is alive"
 * pulse -- or every HB_FAST_PERIOD_MS while the detector sees a hand. */
void heartbeat_task(void *arg)
{
	bool led_on = false;

	hb_task = xTaskGetCurrentTaskHandle();
	while (1) {
		ulTaskNotifyTake(pdTRUE, pdMS_TO_TICKS(fast ? HB_FAST_PERIOD_MS : HB_PERIOD_MS));
		led_on = !led_on;
		led_set(led_on);
	}
}
