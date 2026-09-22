#include <stdint.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_adc/adc_oneshot.h"
#include "esp_log.h"

#include "node_config.h"
#include "can.h"
#include "metrics.h"
#include "pot_sender_role.h"

static const char *TAG = "pot_sender";

/* Scratch ID in the unallocated gap (0x100-0x6FF) -- see
 * ../../../docs/can-message-spec.md. Not a real registered message.
 * Must match pot_collector_role.c's copy of the same define. */
#define CAN_ID_POT_VALUE 0x130

static adc_oneshot_unit_handle_t adc_handle;

/* Raw 12-bit reading (0-4095 at the default ADC_ATTEN_DB_12, spanning
 * roughly the full 0-3.3V input range) -- no calibration to millivolts,
 * on purpose: this is meant as the simplest possible analog-input
 * exercise, and raw counts are enough to prove the read path works. */
static void pot_task(void *arg)
{
	while (1) {
		int raw;
		if (adc_oneshot_read(adc_handle, POT_ADC_CHANNEL, &raw) == ESP_OK) {
			metrics_set_int("pot_sender.raw", raw);
			can_send_u32(CAN_ID_POT_VALUE, (uint32_t)raw);
		} else {
			ESP_LOGW(TAG, "adc read failed");
		}

		vTaskDelay(pdMS_TO_TICKS(POT_SEND_PERIOD_MS));
	}
}

void launch_pot_sender_role(void)
{
	adc_oneshot_unit_init_cfg_t init_config = {
		.unit_id = POT_ADC_UNIT,
	};
	if (adc_oneshot_new_unit(&init_config, &adc_handle) != ESP_OK) {
		ESP_LOGE(TAG, "init: failed to create ADC unit");
		return;
	}

	adc_oneshot_chan_cfg_t chan_config = {
		.bitwidth = ADC_BITWIDTH_DEFAULT,
		.atten = ADC_ATTEN_DB_12,
	};
	if (adc_oneshot_config_channel(adc_handle, POT_ADC_CHANNEL, &chan_config) != ESP_OK) {
		ESP_LOGE(TAG, "init: failed to configure ADC channel");
		return;
	}

	metrics_register_int("pot_sender.raw", 0);

	xTaskCreate(pot_task, "pot_sender", 3072, NULL, 5, NULL);
}
