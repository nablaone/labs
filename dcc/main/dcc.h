#pragma once
#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

#include "freertos/FreeRTOS.h"

/*
 * DCC signal capture and packet decoding -- the receive half of a DCC
 * decoder, see ../docs/dcc-accessory-primer.md sections 1-2.
 *
 * Same shape as canbus's can.c: one producer fans every received packet
 * out to one software queue that any consumer drains via dcc_receive()
 * (today the "dcc sniff" CLI command; later the accessory layer). The
 * difference is that there's no hardware controller doing the bit-level
 * work for us the way TWAI does for CAN -- here the producer is a GPIO
 * edge interrupt on DCC_IN_GPIO that timestamps every edge, classifies
 * each half-bit by its duration, pairs half-bits into bits, and runs the
 * preamble/byte/end-bit state machine itself. Only whole, checksum-
 * verified packets leave the ISR.
 */

/* S-9.2.1 packets are at most 6 bytes including the checksum; a little
 * headroom so a slightly longer (advanced/unknown) packet is still
 * captured rather than counted as an overflow. */
#define DCC_MAX_BYTES 8

typedef struct {
	int64_t time_us;               /* esp_timer time of the packet end bit */
	uint8_t len;                   /* bytes in data[], including the XOR checksum */
	uint8_t data[DCC_MAX_BYTES];
} dcc_packet_t;

/* Configures DCC_IN_GPIO as an any-edge interrupt input and creates the
 * packet queue. Decoding starts immediately. */
void dcc_init(void);

/* Blocks up to `timeout` for the next checksum-verified packet. Returns
 * false on timeout. Like can_receive(), there's one queue: two consumers
 * would split the packets between them. */
bool dcc_receive(dcc_packet_t *pkt, TickType_t timeout);

/* Human-readable one-line description of a packet ("IDLE",
 * "LOCO 3 speed 12/28 fwd", "ACC 1 R=1 straight ON (wire 4)", ...). */
void dcc_describe(const dcc_packet_t *pkt, char *buf, size_t size);

/* Registers the "dcc" CLI command (sniff/stats/timing subcommands). */
void dcc_register_cli_commands(void);
