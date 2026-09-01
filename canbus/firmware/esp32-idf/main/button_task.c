#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "driver/gpio.h"

#include "node_config.h"
#include "state.h"
#include "button_task.h"
#if NODE_ENABLE_CAN
#include "can.h"
#endif

/* Scratch ID for the button-press event -- see display_task.c's
 * DISPLAY_CAN_ID comment for the general ID-choice reasoning. Lower than
 * DISPLAY_CAN_ID (0x7F0) since a button press is a more immediate event
 * than periodic telemetry, but still in the unallocated gap
 * ../../../docs/can-message-spec.md leaves open (0x100-0x6FF), not a
 * real registered message. */
#define BUTTON_CAN_ID 0x110

void button_task_init(void)
{
	gpio_reset_pin(BUTTON_GPIO);
	gpio_set_direction(BUTTON_GPIO, GPIO_MODE_INPUT);
	gpio_pullup_en(BUTTON_GPIO);
	gpio_pulldown_dis(BUTTON_GPIO);
}

/* Polls the button every 100ms; increments the counter -- and, if CAN is
 * enabled, broadcasts its new value on BUTTON_CAN_ID -- on every poll
 * where it reads pressed, so holding it down keeps incrementing/sending
 * rather than counting/sending only on the initial press. */
void button_task(void *arg)
{
	while (1) {
		/* active-low: button pressed means the pin reads 0 */
		if (gpio_get_level(BUTTON_GPIO) == 0) {
			state_counter_increment();

#if NODE_ENABLE_CAN
			can_send_u32(BUTTON_CAN_ID, state_counter_read());
#endif
		}

		vTaskDelay(pdMS_TO_TICKS(POLL_MS));
	}
}
