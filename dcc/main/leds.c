#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>

#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"
#include "freertos/semphr.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_console.h"
#include "esp_rom_sys.h"
#include "driver/rmt_tx.h"
#include "driver/rmt_rx.h"
#include "esp_attr.h"

#include "node_config.h"
#include "leds.h"

static const char *TAG = "leds";

/* RMT tick = 0.1 us, so the bit timings below are in tenths of a us. */
#define RMT_RES_HZ 10000000

/*
 * Bit timing. WS2812 wants 0.4/0.85 us and PL9823 0.35/1.36 us, each
 * +-0.15 us -- but in practice both chips just sample DIN ~0.6 us after
 * the rising edge: a pulse that has already ended means "0", one still
 * high means "1". 0.4 / 0.9 us sits safely on both sides of that.
 */
#define T_SHORT 4   /* 0.4 us */
#define T_LONG  9   /* 0.9 us */

/* Low time that latches the frame. Old WS2812s need 50 us, newer
 * WS2812B/PL9823 batches up to 280 us; 300 us covers all of them. */
#define RESET_US 300

static rmt_channel_handle_t chan;
static rmt_encoder_handle_t encoder;
static SemaphoreHandle_t lock;   /* CLI task and NimBLE host task both call in */

static led_state_t leds[LEDS_COUNT];
static uint8_t brightness = LEDS_DEFAULT_BRIGHTNESS;
static bool order_rgb = LEDS_ORDER_RGB;
static void (*change_cb)(void);

/* Builds the wire frame: colour order and brightness applied, 3 bytes
 * per LED. Caller holds `lock`. */
static void build_frame(uint8_t *frame)
{
	for (int i = 0; i < LEDS_COUNT; i++) {
		uint8_t r = 0, g = 0, b = 0;
		if (leds[i].on) {
			/* +127 rounds instead of truncating, so low
			 * brightness doesn't zero out dim channels early. */
			r = (leds[i].r * brightness + 127) / 255;
			g = (leds[i].g * brightness + 127) / 255;
			b = (leds[i].b * brightness + 127) / 255;
		}
		/* WS2812 expects green first, PL9823 red first. Sent MSB
		 * first (the encoder's msb_first flag). */
		uint8_t *p = frame + i * 3;
		p[0] = order_rgb ? r : g;
		p[1] = order_rgb ? g : r;
		p[2] = b;
	}
}

/* Sends a frame and latches it. Caller holds `lock`. */
static void send_frame_locked(const uint8_t *frame, size_t len)
{
	rmt_transmit_config_t tx = { .loop_count = 0 };
	esp_err_t err = rmt_transmit(chan, encoder, frame, len, &tx);
	if (err == ESP_OK) {
		/* rmt_transmit() only queues the frame and returns; wait
		 * for it to leave (frame is on the caller's stack), then hold
		 * the line low long enough to latch before anyone can send
		 * again. */
		rmt_tx_wait_all_done(chan, portMAX_DELAY);
		esp_rom_delay_us(RESET_US);
	} else {
		ESP_LOGE(TAG, "transmit: %s", esp_err_to_name(err));
	}
}

/* Sends the current state to the chain. Caller holds `lock`. */
static void show_locked(void)
{
	uint8_t frame[LEDS_COUNT * 3];
	build_frame(frame);
	send_frame_locked(frame, sizeof(frame));
}

static void changed(void)
{
	if (change_cb) {
		change_cb();
	}
}

void leds_init(void)
{
	lock = xSemaphoreCreateMutex();

	rmt_tx_channel_config_t ch_cfg = {
		.gpio_num = LEDS_GPIO,
		.clk_src = RMT_CLK_SRC_DEFAULT,
		.resolution_hz = RMT_RES_HZ,
		/* 48 symbols is one RMT memory block on the C3; the driver
		 * refills it from the encoder by interrupt as it drains. */
		.mem_block_symbols = 48,
		.trans_queue_depth = 4,
	};
	ESP_ERROR_CHECK(rmt_new_tx_channel(&ch_cfg, &chan));

	/* The bytes encoder turns every bit of the buffer into one RMT
	 * symbol: (level0 for duration0, then level1 for duration1). */
	rmt_bytes_encoder_config_t enc_cfg = {
		.bit0 = { .level0 = 1, .duration0 = T_SHORT, .level1 = 0, .duration1 = T_LONG },
		.bit1 = { .level0 = 1, .duration0 = T_LONG,  .level1 = 0, .duration1 = T_SHORT },
		.flags.msb_first = 1,
	};
	ESP_ERROR_CHECK(rmt_new_bytes_encoder(&enc_cfg, &encoder));
	ESP_ERROR_CHECK(rmt_enable(chan));

	for (int i = 0; i < LEDS_COUNT; i++) {
		leds[i] = (led_state_t){ .on = false, .r = 255, .g = 255, .b = 255 };
	}
	xSemaphoreTake(lock, portMAX_DELAY);
	show_locked();
	xSemaphoreGive(lock);

	ESP_LOGI(TAG, "%d LEDs on GPIO%d, %s order", LEDS_COUNT, LEDS_GPIO,
		 order_rgb ? "RGB" : "GRB");
}

bool leds_set_on(uint8_t index, bool on)
{
	if (index != LEDS_ALL && index >= LEDS_COUNT) {
		return false;
	}
	xSemaphoreTake(lock, portMAX_DELAY);
	for (int i = 0; i < LEDS_COUNT; i++) {
		if (index == LEDS_ALL || index == i) {
			leds[i].on = on;
		}
	}
	show_locked();
	xSemaphoreGive(lock);
	changed();
	return true;
}

bool leds_set_color(uint8_t index, uint8_t r, uint8_t g, uint8_t b)
{
	if (index != LEDS_ALL && index >= LEDS_COUNT) {
		return false;
	}
	xSemaphoreTake(lock, portMAX_DELAY);
	for (int i = 0; i < LEDS_COUNT; i++) {
		if (index == LEDS_ALL || index == i) {
			leds[i] = (led_state_t){ .on = true, .r = r, .g = g, .b = b };
		}
	}
	show_locked();
	xSemaphoreGive(lock);
	changed();
	return true;
}

void leds_set_brightness(uint8_t value)
{
	xSemaphoreTake(lock, portMAX_DELAY);
	brightness = value;
	show_locked();
	xSemaphoreGive(lock);
	changed();
}

uint8_t leds_count(void)
{
	return LEDS_COUNT;
}

uint8_t leds_brightness(void)
{
	return brightness;
}

led_state_t leds_get(uint8_t index)
{
	led_state_t s = { 0 };
	if (index < LEDS_COUNT) {
		xSemaphoreTake(lock, portMAX_DELAY);
		s = leds[index];
		xSemaphoreGive(lock);
	}
	return s;
}

void leds_set_change_cb(void (*cb)(void))
{
	change_cb = cb;
}

/* ---- CLI ---- */

static const struct {
	const char *name;
	uint8_t r, g, b;
} colors[] = {
	{ "red",     255,   0,   0 },
	{ "green",     0, 255,   0 },
	{ "blue",      0,   0, 255 },
	{ "white",   255, 255, 255 },
	{ "yellow",  255, 160,   0 },
	{ "orange",  255,  60,   0 },
	{ "cyan",      0, 255, 255 },
	{ "magenta", 255,   0, 255 },
};

static void print_state(void)
{
	printf("%d LEDs on GPIO%d, %s order, brightness %u/255\n",
	       LEDS_COUNT, LEDS_GPIO, order_rgb ? "RGB" : "GRB", brightness);
	for (int i = 0; i < LEDS_COUNT; i++) {
		led_state_t s = leds_get(i);
		printf("  %2d  %-3s  #%02X%02X%02X\n", i, s.on ? "on" : "off", s.r, s.g, s.b);
	}
}

/* Parses "0".."LEDS_COUNT-1" or "all". */
static bool parse_index(const char *s, uint8_t *out)
{
	if (strcasecmp(s, "all") == 0) {
		*out = LEDS_ALL;
		return true;
	}
	char *end;
	long v = strtol(s, &end, 10);
	if (*end != '\0' || v < 0 || v >= LEDS_COUNT) {
		return false;
	}
	*out = (uint8_t)v;
	return true;
}

static bool parse_byte(const char *s, uint8_t *out)
{
	char *end;
	long v = strtol(s, &end, 0);
	if (*end != '\0' || v < 0 || v > 255) {
		return false;
	}
	*out = (uint8_t)v;
	return true;
}

/*
 * Lights each LED red, green, blue in turn, then leaves them off. What
 * to look for:
 *  - LED n lights up as the n-th one along the chain -> DIN/DOUT and
 *    the chain order are right.
 *  - Red and green swapped -> wrong colour order, try "led order rgb".
 *  - Chain stops at some LED -> that LED's DOUT->next DIN link (or the
 *    next LED's power) is the problem.
 */
static int cmd_led_test(void)
{
	static const uint8_t seq[3][3] = { { 255, 0, 0 }, { 0, 255, 0 }, { 0, 0, 255 } };
	static const char *names[3] = { "red", "green", "blue" };

	leds_set_on(LEDS_ALL, false);
	for (int i = 0; i < LEDS_COUNT; i++) {
		for (int c = 0; c < 3; c++) {
			printf("LED %d %s\n", i, names[c]);
			leds_set_color(i, seq[c][0], seq[c][1], seq[c][2]);
			vTaskDelay(pdMS_TO_TICKS(400));
		}
		leds_set_on(i, false);
	}
	return 0;
}

/*
 * ---- led probe: loopback self-test ----
 *
 * Proves the bit stream really leaves GPIO5, without a scope: jumper
 * LEDS_GPIO to LEDS_PROBE_GPIO and let the RMT peripheral listen to
 * itself. RMT has receive channels as well as transmit ones: an RX
 * channel timestamps every edge on its pin in hardware (here at the same
 * 0.1 us tick as TX) and stores each high+low pair as one "symbol" --
 * exactly the format the TX side sends. GPIO interrupts couldn't do
 * this: entering an ISR takes a few us, longer than a whole bit. (The C3
 * also has no PCNT pulse counter, so RMT is the tool here.)
 *
 * The probe sends a fixed test pattern rather than the current state,
 * because with everything off the frame would be all "0" bits and the
 * long-pulse "1" would never be checked. The real state is re-sent
 * straight after, so the LEDs show the pattern for about a millisecond
 * -- too short to see.
 *
 * The RX channel is only created on the first probe, so normal running
 * doesn't spend a channel or a pin on it.
 */
#define PROBE_BITS (LEDS_COUNT * 24)

static rmt_channel_handle_t probe_chan;
static QueueHandle_t probe_queue;
/* Room for a few glitch symbols beyond the frame. Static: the driver
 * fills it from its ISR after rmt_receive() returns. */
static rmt_symbol_word_t probe_buf[PROBE_BITS + 16];

/* RMT ISR context: hand the symbol count to the waiting CLI task. */
static bool IRAM_ATTR probe_done(rmt_channel_handle_t ch,
				 const rmt_rx_done_event_data_t *ev, void *arg)
{
	BaseType_t woken = pdFALSE;
	size_t n = ev->num_symbols;
	xQueueSendFromISR((QueueHandle_t)arg, &n, &woken);
	return woken == pdTRUE;
}

static esp_err_t probe_setup(void)
{
	if (probe_chan) {
		return ESP_OK;
	}
	rmt_rx_channel_config_t cfg = {
		.gpio_num = LEDS_PROBE_GPIO,
		.clk_src = RMT_CLK_SRC_DEFAULT,
		.resolution_hz = RMT_RES_HZ,
		/* One 48-symbol memory block; the frame is longer, so the
		 * driver empties the block into probe_buf half by half
		 * ("ping-pong") as it fills. */
		.mem_block_symbols = 48,
	};
	esp_err_t err = rmt_new_rx_channel(&cfg, &probe_chan);
	if (err != ESP_OK) {
		probe_chan = NULL;
		return err;
	}
	/* Without the jumper the pin would float and pick up noise;
	 * pulled down it reads a clean "nothing arrived". */
	gpio_pulldown_en(LEDS_PROBE_GPIO);
	probe_queue = xQueueCreate(1, sizeof(size_t));
	rmt_rx_event_callbacks_t cbs = { .on_recv_done = probe_done };
	ESP_ERROR_CHECK(rmt_rx_register_event_callbacks(probe_chan, &cbs, probe_queue));
	return rmt_enable(probe_chan);
}

static void print_hist(const char *name, const int *hist, int bins)
{
	printf("  %s:", name);
	for (int t = 0; t < bins; t++) {
		if (hist[t]) {
			printf("  %s%d.%d us x%d", t == bins - 1 ? ">=" : "", t / 10, t % 10, hist[t]);
		}
	}
	printf("\n");
}

static int cmd_led_probe(void)
{
	esp_err_t err = probe_setup();
	if (err != ESP_OK) {
		printf("probe: can't set up RMT receive on GPIO%d: %s\n",
		       LEDS_PROBE_GPIO, esp_err_to_name(err));
		return 1;
	}

	/* Pattern bytes: runs of 1s, runs of 0s and alternating bits. */
	static const uint8_t pat[3] = { 0xF0, 0xCC, 0xAA };
	uint8_t frame[LEDS_COUNT * 3];
	for (size_t i = 0; i < sizeof(frame); i++) {
		frame[i] = pat[i % 3];
	}

	/* min: anything shorter than 0.1 us is a glitch, not a bit.
	 * max: once the line has been quiet for 20 us the frame is over
	 * (bits are 1.3 us, the latch gap is 300 us). The receiver waits
	 * for the first edge before it starts timing. */
	rmt_receive_config_t rx = {
		.signal_range_min_ns = 100,
		.signal_range_max_ns = 20000,
	};
	xQueueReset(probe_queue);
	xSemaphoreTake(lock, portMAX_DELAY);
	err = rmt_receive(probe_chan, probe_buf, sizeof(probe_buf), &rx);
	if (err == ESP_OK) {
		send_frame_locked(frame, sizeof(frame));
		show_locked();   /* put the real state back */
	}
	xSemaphoreGive(lock);
	if (err != ESP_OK) {
		printf("probe: rmt_receive: %s\n", esp_err_to_name(err));
		return 1;
	}

	size_t n = 0;
	if (xQueueReceive(probe_queue, &n, pdMS_TO_TICKS(50)) != pdTRUE) {
		/* Still armed, waiting for an edge that never came:
		 * disable/enable cancels it for the next try. */
		rmt_disable(probe_chan);
		rmt_enable(probe_chan);
		printf("sent %d bits on GPIO%d, nothing arrived on GPIO%d.\n"
		       "Is the jumper GPIO%d -> GPIO%d in place?\n",
		       PROBE_BITS, LEDS_GPIO, LEDS_PROBE_GPIO, LEDS_GPIO, LEDS_PROBE_GPIO);
		return 1;
	}

	/* Histogram of high and low times, in 0.1 us ticks. The last
	 * symbol's low part is the idle line after the frame, not a bit,
	 * so it's left out. */
	enum { BINS = 16 };
	int hi[BINS] = { 0 }, lo[BINS] = { 0 };
	int bad_bits = 0, first_bad = -1, wrong_level = 0;
	for (size_t i = 0; i < n; i++) {
		rmt_symbol_word_t s = probe_buf[i];
		if (s.level0 != 1) {
			wrong_level++;
			continue;
		}
		hi[s.duration0 < BINS ? s.duration0 : BINS - 1]++;
		if (i + 1 < n) {
			lo[s.duration1 < BINS ? s.duration1 : BINS - 1]++;
		}
		/* Decide the bit the way the LED does: is the pulse
		 * longer than halfway between "0" and "1"? */
		int bit = 2 * s.duration0 > T_SHORT + T_LONG;
		if (i < PROBE_BITS) {
			int want = (frame[i / 8] >> (7 - i % 8)) & 1;
			if (bit != want) {
				if (first_bad < 0) {
					first_bad = i;
				}
				bad_bits++;
			}
		}
	}

	printf("sent %d bits on GPIO%d, captured %u on GPIO%d\n",
	       PROBE_BITS, LEDS_GPIO, (unsigned)n, LEDS_PROBE_GPIO);
	print_hist("high", hi, BINS);
	print_hist("low ", lo, BINS);
	printf("  (want high 0.4 us = \"0\", 0.9 us = \"1\", lows the other way round)\n");

	if (wrong_level) {
		printf("%d pulses started low: inverted signal?\n", wrong_level);
	}
	if (n == PROBE_BITS && bad_bits == 0 && wrong_level == 0) {
		printf("OK: all %d bits match, the data leaves GPIO%d correctly\n",
		       PROBE_BITS, LEDS_GPIO);
		return 0;
	}
	if (bad_bits) {
		printf("%d bits wrong, first at bit %d (LED %d, byte %d)\n",
		       bad_bits, first_bad, first_bad / 24, first_bad / 8 % 3);
	}
	if (n != PROBE_BITS) {
		printf("bit count mismatch: loose jumper or noise?\n");
	}
	return 1;
}

static int usage(void)
{
	printf("usage:\n"
	       "  led                       show state\n"
	       "  led <n|all> on|off\n"
	       "  led <n|all> <colour>      red green blue white yellow orange cyan magenta\n"
	       "  led <n|all> <r> <g> <b>   0-255 each\n"
	       "  led bright <0-255>\n"
	       "  led order grb|rgb         WS2812 = grb, PL9823 = rgb\n"
	       "  led test                  walk red/green/blue along the chain\n"
	       "  led probe                 loopback self-test, jumper GPIO%d -> GPIO%d\n",
	       LEDS_GPIO, LEDS_PROBE_GPIO);
	return 1;
}

static int cmd_led(int argc, char **argv)
{
	if (argc == 1) {
		print_state();
		return 0;
	}

	if (strcmp(argv[1], "test") == 0) {
		return cmd_led_test();
	}
	if (strcmp(argv[1], "probe") == 0) {
		return cmd_led_probe();
	}
	if (strcmp(argv[1], "bright") == 0) {
		uint8_t v;
		if (argc != 3 || !parse_byte(argv[2], &v)) {
			return usage();
		}
		leds_set_brightness(v);
		return 0;
	}
	if (strcmp(argv[1], "order") == 0) {
		if (argc != 3 || (strcasecmp(argv[2], "rgb") != 0 && strcasecmp(argv[2], "grb") != 0)) {
			return usage();
		}
		xSemaphoreTake(lock, portMAX_DELAY);
		order_rgb = strcasecmp(argv[2], "rgb") == 0;
		show_locked();
		xSemaphoreGive(lock);
		return 0;
	}

	uint8_t idx;
	if (argc < 3 || !parse_index(argv[1], &idx)) {
		return usage();
	}

	if (argc == 3) {
		if (strcasecmp(argv[2], "on") == 0) {
			leds_set_on(idx, true);
			return 0;
		}
		if (strcasecmp(argv[2], "off") == 0) {
			leds_set_on(idx, false);
			return 0;
		}
		for (size_t i = 0; i < sizeof(colors) / sizeof(colors[0]); i++) {
			if (strcasecmp(argv[2], colors[i].name) == 0) {
				leds_set_color(idx, colors[i].r, colors[i].g, colors[i].b);
				return 0;
			}
		}
		return usage();
	}

	uint8_t r, g, b;
	if (argc == 5 && parse_byte(argv[2], &r) && parse_byte(argv[3], &g) && parse_byte(argv[4], &b)) {
		leds_set_color(idx, r, g, b);
		return 0;
	}
	return usage();
}

void leds_register_cli_commands(void)
{
	const esp_console_cmd_t led_cmd = {
		.command = "led",
		.help = "Addressable LED chain: 'led' for state, 'led help' for usage",
		.func = &cmd_led,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&led_cmd));
}
