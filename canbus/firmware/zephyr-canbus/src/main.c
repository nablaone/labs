#include <zephyr/kernel.h>
#include <zephyr/device.h>
#include <zephyr/drivers/gpio.h>
#include <zephyr/sys/printk.h>

#define SLOW_BLINK_MS 500
#define FAST_BLINK_MS 100
#define POLL_MS 10

static const struct gpio_dt_spec led = GPIO_DT_SPEC_GET(DT_ALIAS(led0), gpios);
static const struct gpio_dt_spec button = GPIO_DT_SPEC_GET(DT_ALIAS(sw0), gpios);

#ifdef CONFIG_GPIO_EMUL
/*
 * native_sim has no real button to press, so auto-toggle the emulated
 * button's input signal periodically -- purely to demo the blink-rate
 * change without hardware. Real boards don't compile this in.
 */
#include <zephyr/drivers/gpio/gpio_emul.h>

static void button_sim_work_handler(struct k_work *work)
{
	static bool pressed;

	pressed = !pressed;
	/* active-low: "pressed" means driving the pin low */
	gpio_emul_input_set(button.port, button.pin, pressed ? 0 : 1);
	printk("[sim] button %s\n", pressed ? "pressed" : "released");

	k_work_reschedule(k_work_delayable_from_work(work), K_SECONDS(3));
}

static K_WORK_DELAYABLE_DEFINE(button_sim_work, button_sim_work_handler);
#endif

int main(void)
{
	if (!gpio_is_ready_dt(&led) || !gpio_is_ready_dt(&button)) {
		return -ENODEV;
	}

	gpio_pin_configure_dt(&led, GPIO_OUTPUT_INACTIVE);
	gpio_pin_configure_dt(&button, GPIO_INPUT);

#ifdef CONFIG_GPIO_EMUL
	printk("[sim] native_sim: no real button, auto-toggling every 3s "
	       "to demo the blink-rate change\n");
	k_work_reschedule(&button_sim_work, K_SECONDS(3));
#endif

	int32_t elapsed_ms = 0;
	bool led_on = false;

	while (1) {
		int period_ms = gpio_pin_get_dt(&button) ? FAST_BLINK_MS : SLOW_BLINK_MS;

		elapsed_ms += POLL_MS;
		if (elapsed_ms >= period_ms) {
			led_on = !led_on;
			gpio_pin_set_dt(&led, led_on);
			printk("led %s (period=%dms)\n", led_on ? "on" : "off", period_ms);
			elapsed_ms = 0;
		}

		k_msleep(POLL_MS);
	}

	return 0;
}
