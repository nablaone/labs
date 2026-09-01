#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"

#include "node_config.h"
#include "state.h"
#include "display_task.h"

static const char *TAG = "display";

void display_task_init(void)
{
	/* Nothing to set up -- just logs on a timer. */
}

/* Logs the excitement counter's value every DISPLAY_MS (suppressed along
 * with everything else while CLI mode has the log level muted). */
void display_task(void *arg)
{
	while (1) {
		vTaskDelay(pdMS_TO_TICKS(DISPLAY_MS));
		ESP_LOGI(TAG, "excitement counter = %" PRIu32, state_counter_read());
	}
}
