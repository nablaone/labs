#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <inttypes.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/queue.h"
#include "driver/gpio.h"
#include "esp_attr.h"
#include "esp_timer.h"
#include "esp_log.h"
#include "esp_console.h"

#include "node_config.h"
#include "dcc.h"

static const char *TAG = "dcc";

/*
 * Half-bit classification windows, in microseconds. S-9.1 Table 2.1 says
 * a decoder must accept "1" halves of 52-64 us and "0" halves of
 * 90-10000 us. The windows here are deliberately wider on the inside
 * (split at 77 us, between the two ranges) to absorb GPIO interrupt
 * latency jitter, and the "0" maximum covers a stretched zero's 12 ms
 * total. Anything outside both is noise or a gap (e.g. a RailCom cutout)
 * and forces a resync.
 */
#define HALF_ONE_MIN_US   35
#define HALF_ONE_MAX_US   77
#define HALF_ZERO_MAX_US  12000

/* S-9.2: a decoder must not accept fewer than 10 preamble "1" bits. */
#define DCC_PREAMBLE_MIN 10

/* Generous: a command station sends a few hundred packets a second, and
 * "dcc sniff" may be slow printing over USB. When nobody is consuming,
 * the queue simply fills up and further packets are dropped (counted in
 * "dcc stats" as queue_full -- expected while not sniffing). */
#define DCC_RX_QUEUE_LEN 64

#define DCC_SNIFF_DEFAULT_S 30
#define DCC_TIMING_DEFAULT_MS 1000

/* "dcc timing" histogram: 10 us buckets 0-199 us, plus one for >= 200. */
#define HIST_BUCKET_US 10
#define HIST_BUCKETS   21

static QueueHandle_t dcc_rx_queue;

/* Written only by the ISR, read (and cleared) by CLI commands. Plain
 * 32-bit counters: single-word reads/writes are atomic on the C3, and an
 * occasionally lost increment during "dcc stats clear" doesn't matter. */
static volatile struct {
	uint32_t edges;
	uint32_t packets_ok;
	uint32_t bad_timing;   /* half-bit outside both windows */
	uint32_t bad_pair;     /* the two halves of a bit disagreed */
	uint32_t cutouts;      /* either of the above right after a packet:
				* a RailCom cutout, expected on PIKO SmartControl */
	uint32_t bad_xor;      /* checksum mismatch */
	uint32_t too_short;    /* end bit after fewer than 3 bytes */
	uint32_t overflow;     /* more than DCC_MAX_BYTES bytes */
	uint32_t queue_full;
} stats;

static volatile uint32_t hist[HIST_BUCKETS];

/* ---- ISR-side decoder state (touched only from dcc_edge_isr) ---- */

typedef enum { PKT_PREAMBLE, PKT_BYTE, PKT_SEPARATOR } pkt_state_t;

static int64_t last_edge_us;

/* Half-bit layer. Before sync, we only count consecutive "1" halves; the
 * first "0" half after at least 2 * DCC_PREAMBLE_MIN of them is, by
 * definition, the first half of the packet start bit -- that's how the
 * decoder learns which edge starts a bit without knowing the rail
 * polarity. From then on halves are taken in pairs. */
static bool synced;
static uint32_t one_halves;
static int pending_half = -1;  /* -1: none, else 0/1 = value of first half */

/* Packet layer. */
static pkt_state_t pkt_state;
static uint32_t preamble_ones;
static uint8_t cur_byte;
static uint8_t bit_count;
static dcc_packet_t pkt;

/* Set when a packet ends, cleared two preamble bits later. A RailCom
 * cutout (the command station shorting the rails for ~450 us right after
 * the end bit, so decoders can talk back) mangles exactly that stretch,
 * so a sync loss inside it is expected rather than an error. */
static bool after_packet;

static void IRAM_ATTR pkt_reset(void)
{
	pkt_state = PKT_PREAMBLE;
	preamble_ones = 0;
}

/* Called on the *first* half of the end bit rather than once the whole
 * bit is in: with RailCom (which PIKO SmartControl uses), the command
 * station cuts the track signal right after the end bit, so its second
 * half can be stretched into the cutout and come out as garbage. The
 * packet is complete at this point anyway. */
static void IRAM_ATTR pkt_end(int64_t now, BaseType_t *woken)
{
	if (pkt.len < 3) {
		stats.too_short++;
		return;
	}

	uint8_t x = 0;
	for (int i = 0; i < pkt.len; i++) {
		x ^= pkt.data[i];
	}
	if (x != 0) {
		stats.bad_xor++;
		return;
	}

	stats.packets_ok++;
	pkt.time_us = now;
	if (xQueueSendFromISR(dcc_rx_queue, &pkt, woken) != pdTRUE) {
		stats.queue_full++;
	}
}

static void IRAM_ATTR on_bit(int bit)
{
	switch (pkt_state) {
	case PKT_PREAMBLE:
		if (bit) {
			if (++preamble_ones >= 2) {
				after_packet = false;
			}
		} else if (preamble_ones >= DCC_PREAMBLE_MIN) {
			/* packet start bit */
			pkt_state = PKT_BYTE;
			pkt.len = 0;
			cur_byte = 0;
			bit_count = 0;
		} else {
			preamble_ones = 0;
		}
		break;

	case PKT_BYTE:
		cur_byte = (uint8_t)((cur_byte << 1) | bit);  /* MSB first */
		if (++bit_count == 8) {
			if (pkt.len == DCC_MAX_BYTES) {
				stats.overflow++;
				pkt_reset();
			} else {
				pkt.data[pkt.len++] = cur_byte;
				pkt_state = PKT_SEPARATOR;
			}
			cur_byte = 0;
			bit_count = 0;
		}
		break;

	case PKT_SEPARATOR:
		/* A "0" is a data byte start bit. A "1" (end bit) never gets
		 * here -- dcc_edge_isr() handles it on its first half. */
		pkt_state = PKT_BYTE;
		break;
	}
}

/* Drops back to hunting for a preamble. Errors while synced are counted
 * here, either as the given stat or as a cutout (see after_packet). */
static void IRAM_ATTR lose_sync(volatile uint32_t *stat)
{
	if (synced) {
		(*(after_packet ? &stats.cutouts : stat))++;
	}
	after_packet = false;
	synced = false;
	one_halves = 0;
	pending_half = -1;
	pkt_reset();
}

/*
 * Fires on every edge of the opto output. The time since the previous
 * edge is one half-bit, whichever polarity it had -- the decoder never
 * needs to know which rail is positive.
 */
static void IRAM_ATTR dcc_edge_isr(void *arg)
{
	int64_t now = esp_timer_get_time();
	uint32_t dt = (uint32_t)(now - last_edge_us);
	last_edge_us = now;
	stats.edges++;

	uint32_t bucket = dt / HIST_BUCKET_US;
	hist[bucket < HIST_BUCKETS ? bucket : HIST_BUCKETS - 1]++;

	int half;
	if (dt >= HALF_ONE_MIN_US && dt <= HALF_ONE_MAX_US) {
		half = 1;
	} else if (dt > HALF_ONE_MAX_US && dt <= HALF_ZERO_MAX_US) {
		half = 0;
	} else {
		lose_sync(&stats.bad_timing);
		return;
	}

	if (!synced) {
		if (half) {
			one_halves++;
			return;
		}
		if (one_halves >= 2 * DCC_PREAMBLE_MIN) {
			/* first half of the start bit */
			synced = true;
			pkt_state = PKT_PREAMBLE;
			preamble_ones = one_halves / 2;
			pending_half = 0;
		}
		one_halves = 0;
		return;
	}

	BaseType_t woken = pdFALSE;

	if (pending_half < 0) {
		pending_half = half;
		if (half && pkt_state == PKT_SEPARATOR) {
			/* first half of the packet end bit */
			pkt_end(now, &woken);
			pkt_reset();
			after_packet = true;
		}
	} else if (half != pending_half) {
		lose_sync(&stats.bad_pair);
		one_halves = half;
	} else {
		on_bit(half);
		pending_half = -1;
	}

	if (woken) {
		portYIELD_FROM_ISR();
	}
}

/* ---- packet description ---- */

/* Instruction byte(s) of a multifunction (loco) packet, S-9.2 / S-9.2.1
 * section 2.3. `p` excludes the address and the checksum. Only the
 * common instructions are spelled out; the rest are named by group. */
static void describe_loco_instr(const uint8_t *p, int n, char *buf, size_t size)
{
	if (n < 1) {
		snprintf(buf, size, "(no instruction)");
		return;
	}

	uint8_t b = p[0];

	if (b == 0x3F && n >= 2) {
		/* 128 speed steps: 00111111 DSSSSSSS */
		const char *dir = (p[1] & 0x80) ? "fwd" : "rev";
		int s = p[1] & 0x7F;
		if (s == 0) {
			snprintf(buf, size, "speed STOP %s", dir);
		} else if (s == 1) {
			snprintf(buf, size, "speed E-STOP %s", dir);
		} else {
			snprintf(buf, size, "speed %d/126 %s", s - 1, dir);
		}
	} else if ((b & 0xC0) == 0x40) {
		/* 28 speed steps: 01DCSSSS, C is the speed LSB (S-9.2 Fig. 2) */
		const char *dir = (b & 0x20) ? "fwd" : "rev";
		int v = ((b & 0x0F) << 1) | ((b >> 4) & 1);
		if (v <= 1) {
			snprintf(buf, size, "speed STOP %s", dir);
		} else if (v <= 3) {
			snprintf(buf, size, "speed E-STOP %s", dir);
		} else {
			snprintf(buf, size, "speed %d/28 %s", v - 3, dir);
		}
	} else if ((b & 0xE0) == 0x80) {
		/* 100DDDDD: F0 in bit 4, F1-F4 in bits 0-3 */
		snprintf(buf, size, "F0=%d F1-F4=%d%d%d%d", (b >> 4) & 1,
			 b & 1, (b >> 1) & 1, (b >> 2) & 1, (b >> 3) & 1);
	} else if ((b & 0xF0) == 0xB0) {
		snprintf(buf, size, "F5-F8=%d%d%d%d",
			 b & 1, (b >> 1) & 1, (b >> 2) & 1, (b >> 3) & 1);
	} else if ((b & 0xF0) == 0xA0) {
		snprintf(buf, size, "F9-F12=%d%d%d%d",
			 b & 1, (b >> 1) & 1, (b >> 2) & 1, (b >> 3) & 1);
	} else if (b == 0xDE && n >= 2) {
		snprintf(buf, size, "F13-F20=0x%02X", p[1]);
	} else if (b == 0xDF && n >= 2) {
		snprintf(buf, size, "F21-F28=0x%02X", p[1]);
	} else if ((b & 0xF0) == 0xE0) {
		snprintf(buf, size, "CV access (POM)");
	} else if ((b & 0xE0) == 0x00) {
		snprintf(buf, size, "decoder/consist control 0x%02X", b);
	} else {
		snprintf(buf, size, "instruction 0x%02X", b);
	}
}

/* Accessory packets, RCN-213 / primer section 4-5. `d` excludes the
 * checksum. */
static void describe_accessory(const uint8_t *d, int n, char *buf, size_t size)
{
	if (n < 2) {
		snprintf(buf, size, "ACC (truncated)");
		return;
	}

	uint8_t b1 = d[0];
	uint8_t b2 = d[1];
	/* High address bits are sent inverted (one's complement). */
	unsigned wire = ((unsigned)((~b2 >> 4) & 0x07) << 8) |
			((unsigned)(b1 & 0x3F) << 2) |
			((b2 >> 1) & 0x03);
	/* RCN-213 linear numbering: wire 4 is shown as user address 1,
	 * wire 0-3 wrap to the end of the range. */
	unsigned user = wire >= 4 ? wire - 3 : wire + 2045;

	if (b2 & 0x80) {
		if (n == 2) {
			int r = b2 & 0x01;
			snprintf(buf, size, "ACC %u R=%d %s %s (wire %u)", user, r,
				 r ? "straight/green" : "diverging/red",
				 (b2 & 0x08) ? "ON" : "off", wire);
		} else {
			snprintf(buf, size, "ACC %u CV access (POM) (wire %u)", user, wire);
		}
	} else if ((b2 & 0x09) == 0x01 && n >= 3) {
		snprintf(buf, size, "ACC-EXT %u value %u (wire %u)", user, d[2], wire);
	} else if (b2 & 0x08) {
		snprintf(buf, size, "ACC NOP %u (wire %u)", user, wire);
	} else {
		snprintf(buf, size, "ACC %u unknown 0x%02X (wire %u)", user, b2, wire);
	}
}

void dcc_describe(const dcc_packet_t *p, char *buf, size_t size)
{
	const uint8_t *d = p->data;
	int n = p->len - 1;  /* without the checksum */
	char instr[48];

	if (n == 2 && d[0] == 0xFF && d[1] == 0x00) {
		snprintf(buf, size, "IDLE");
	} else if (d[0] == 0x00) {
		if (n == 2 && d[1] == 0x00) {
			snprintf(buf, size, "RESET");
		} else {
			describe_loco_instr(d + 1, n - 1, instr, sizeof(instr));
			snprintf(buf, size, "BROADCAST %s", instr);
		}
	} else if (d[0] <= 0x7F) {
		describe_loco_instr(d + 1, n - 1, instr, sizeof(instr));
		snprintf(buf, size, "LOCO %u %s", d[0], instr);
	} else if (d[0] <= 0xBF) {
		describe_accessory(d, n, buf, size);
	} else if (d[0] <= 0xE7 && n >= 2) {
		unsigned addr = ((unsigned)(d[0] & 0x3F) << 8) | d[1];
		describe_loco_instr(d + 2, n - 2, instr, sizeof(instr));
		snprintf(buf, size, "LOCO %u (long) %s", addr, instr);
	} else {
		snprintf(buf, size, "OTHER");
	}
}

bool dcc_receive(dcc_packet_t *p, TickType_t timeout)
{
	return xQueueReceive(dcc_rx_queue, p, timeout) == pdTRUE;
}

/* ---- CLI ---- */

static bool is_idle(const dcc_packet_t *p)
{
	return p->len == 3 && p->data[0] == 0xFF && p->data[1] == 0x00;
}

/*
 * Command stations repeat every packet: loco speed/function state is
 * refreshed continuously, accessory commands are sent several times in a
 * row. Printing all of that buries the interesting part, so by default
 * a packet is suppressed while an identical one was seen less than
 * REPEAT_WINDOW_US ago -- i.e. each distinct packet prints once, and
 * again only after it has gone quiet for a while. "-a" turns this (and
 * the idle filter) off.
 */
#define RECENT_LEN       16
#define REPEAT_WINDOW_US (1000 * 1000)

static dcc_packet_t recent[RECENT_LEN];
static int recent_next;

static bool seen_recently(const dcc_packet_t *p)
{
	for (int i = 0; i < RECENT_LEN; i++) {
		dcc_packet_t *r = &recent[i];
		if (r->len == p->len && memcmp(r->data, p->data, p->len) == 0) {
			bool repeat = p->time_us - r->time_us < REPEAT_WINDOW_US;
			r->time_us = p->time_us;
			return repeat;
		}
	}
	recent[recent_next] = *p;
	recent_next = (recent_next + 1) % RECENT_LEN;
	return false;
}

/* Prints decoded packets for a fixed time, like canbus's "can sniff":
 * timestamp, raw bytes (checksum included) and the description. */
static int cmd_dcc_sniff(int seconds, bool all)
{
	printf("Sniffing for %ds%s...\n", seconds,
	       all ? " (all packets)" : " (idle and repeats hidden, -a shows all)");

	memset(recent, 0, sizeof(recent));
	recent_next = 0;
	xQueueReset(dcc_rx_queue);
	uint32_t ok_before = stats.packets_ok;
	unsigned printed = 0;

	TickType_t end = xTaskGetTickCount() + pdMS_TO_TICKS(seconds * 1000);
	while (xTaskGetTickCount() < end) {
		dcc_packet_t p;
		if (!dcc_receive(&p, pdMS_TO_TICKS(200))) {
			continue;
		}
		if (!all && (is_idle(&p) || seen_recently(&p))) {
			continue;
		}

		char hex[DCC_MAX_BYTES * 3 + 1] = "";
		for (int i = 0; i < p.len; i++) {
			snprintf(hex + i * 3, sizeof(hex) - i * 3, "%02X ", p.data[i]);
		}
		char desc[80];
		dcc_describe(&p, desc, sizeof(desc));

		printf("%10.3f  %-18s %s\n", p.time_us / 1e6, hex, desc);
		printed++;
	}

	printf("Done. %u printed, %" PRIu32 " valid packets received.\n",
	       printed, stats.packets_ok - ok_before);
	return 0;
}

static int cmd_dcc_stats(bool clear)
{
	if (clear) {
		memset((void *)&stats, 0, sizeof(stats));
		printf("cleared\n");
		return 0;
	}

	printf("edges       %" PRIu32 "\n", stats.edges);
	printf("packets_ok  %" PRIu32 "\n", stats.packets_ok);
	printf("bad_timing  %" PRIu32 "\n", stats.bad_timing);
	printf("bad_pair    %" PRIu32 "\n", stats.bad_pair);
	printf("cutouts     %" PRIu32 " (RailCom, not an error)\n", stats.cutouts);
	printf("bad_xor     %" PRIu32 "\n", stats.bad_xor);
	printf("too_short   %" PRIu32 "\n", stats.too_short);
	printf("overflow    %" PRIu32 "\n", stats.overflow);
	printf("queue_full  %" PRIu32 " (normal while nothing is sniffing)\n", stats.queue_full);
	return 0;
}

/* Hardware bring-up aid: histogram of edge-to-edge times. A healthy
 * opto stage shows one peak at 50-60 us ("1" halves) and one at
 * 90-120 us ("0" halves), and nothing else to speak of. */
static int cmd_dcc_timing(int ms)
{
	printf("Collecting half-bit timings for %dms...\n", ms);
	memset((void *)hist, 0, sizeof(hist));
	vTaskDelay(pdMS_TO_TICKS(ms));

	uint32_t snap[HIST_BUCKETS];
	uint32_t max = 0;
	for (int i = 0; i < HIST_BUCKETS; i++) {
		snap[i] = hist[i];
		if (snap[i] > max) {
			max = snap[i];
		}
	}

	if (max == 0) {
		printf("No edges on GPIO%d -- check the opto wiring and track power.\n",
		       DCC_IN_GPIO);
		return 1;
	}

	for (int i = 0; i < HIST_BUCKETS; i++) {
		if (snap[i] == 0) {
			continue;
		}
		char bar[41];
		int len = (int)((uint64_t)snap[i] * 40 / max);
		memset(bar, '#', len);
		bar[len] = '\0';
		if (i == HIST_BUCKETS - 1) {
			printf("   >=%3d us %7" PRIu32 " %s\n", i * HIST_BUCKET_US, snap[i], bar);
		} else {
			printf("%3d-%3d us %7" PRIu32 " %s\n", i * HIST_BUCKET_US,
			       i * HIST_BUCKET_US + HIST_BUCKET_US - 1, snap[i], bar);
		}
	}
	return 0;
}

static int cmd_dcc(int argc, char **argv)
{
	if (argc < 2) {
		printf("usage: dcc <sniff [seconds] [-a] | stats [clear] | timing [ms]>\n");
		return 1;
	}

	if (strcmp(argv[1], "sniff") == 0) {
		int seconds = DCC_SNIFF_DEFAULT_S;
		bool all = false;
		for (int i = 2; i < argc; i++) {
			if (strcmp(argv[i], "-a") == 0) {
				all = true;
			} else {
				seconds = atoi(argv[i]);
			}
		}
		if (seconds <= 0) {
			printf("bad duration\n");
			return 1;
		}
		return cmd_dcc_sniff(seconds, all);
	} else if (strcmp(argv[1], "stats") == 0) {
		return cmd_dcc_stats(argc >= 3 && strcmp(argv[2], "clear") == 0);
	} else if (strcmp(argv[1], "timing") == 0) {
		int ms = argc >= 3 ? atoi(argv[2]) : DCC_TIMING_DEFAULT_MS;
		if (ms <= 0) {
			printf("bad duration\n");
			return 1;
		}
		return cmd_dcc_timing(ms);
	}

	printf("unknown dcc subcommand: '%s'\n", argv[1]);
	return 1;
}

void dcc_register_cli_commands(void)
{
	const esp_console_cmd_t dcc_cmd = {
		.command = "dcc",
		.help = "DCC: sniff [seconds] [-a] | stats [clear] | timing [ms]",
		.func = &cmd_dcc,
	};
	ESP_ERROR_CHECK(esp_console_cmd_register(&dcc_cmd));
}

void dcc_init(void)
{
	dcc_rx_queue = xQueueCreate(DCC_RX_QUEUE_LEN, sizeof(dcc_packet_t));

	/* The internal pull-up (~45k) sits alongside the external 2.2k one;
	 * it only matters with the opto unplugged, where it keeps the pin
	 * from floating and firing spurious interrupts. */
	const gpio_config_t io_conf = {
		.pin_bit_mask = 1ULL << DCC_IN_GPIO,
		.mode = GPIO_MODE_INPUT,
		.pull_up_en = GPIO_PULLUP_ENABLE,
		.pull_down_en = GPIO_PULLDOWN_DISABLE,
		.intr_type = GPIO_INTR_ANYEDGE,
	};
	ESP_ERROR_CHECK(gpio_config(&io_conf));

	last_edge_us = esp_timer_get_time();
	ESP_ERROR_CHECK(gpio_install_isr_service(0));
	ESP_ERROR_CHECK(gpio_isr_handler_add(DCC_IN_GPIO, dcc_edge_isr, NULL));

	ESP_LOGI(TAG, "listening on GPIO%d", DCC_IN_GPIO);
}
