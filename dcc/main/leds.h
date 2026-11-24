#pragma once
#include <stdbool.h>
#include <stdint.h>

/*
 * Addressable LED chain (5 mm through-hole WS2812D-F5 / PL9823) on
 * LEDS_GPIO -- for now just a wiring test driven from the CLI and the
 * web page; later these become turnout position indicators.
 *
 * How the chain is addressed: there are no addresses on the wire. The
 * ESP sends one 24-bit colour per LED back to back on DIN; each LED
 * keeps the first 24 bits it sees and passes everything after that on
 * through DOUT. So the first colour lands in the LED nearest the ESP
 * (index 0), the second in the next one, and so on. A pause of
 * >= 280 us with the line low ("reset"/latch) makes every LED show
 * what it just received. Sending more colours than there are LEDs is
 * harmless -- the surplus falls off the end of the chain -- so
 * LEDS_COUNT only needs to be >= the real chain length.
 *
 * Bit timing (each bit is a high pulse followed by a low gap, the
 * *length of the high part* encodes the value -- same idea as DCC's
 * half-bit lengths, just 50x faster):
 *   "0"  0.4 us high, 0.9 us low
 *   "1"  0.9 us high, 0.4 us low
 * The ESP32-C3's RMT peripheral generates this in hardware from a
 * buffer of bytes, so the CPU never has to bit-bang sub-microsecond
 * pulses (an interrupt in the middle would corrupt the frame).
 *
 * Each LED has a colour and an on/off flag; "off" keeps the colour so
 * "on" brings it back. A global brightness (0-255) scales everything
 * at send time -- these LEDs are blinding at full power, and it keeps
 * current draw low (~60 mA per LED at full white).
 */

typedef struct {
	bool on;
	uint8_t r, g, b;
} led_state_t;

/* Index meaning "every LED" for leds_set_on()/leds_set_color(). */
#define LEDS_ALL 0xff

/* RMT channel + bytes encoder on LEDS_GPIO, then sends all-off (some
 * LEDs power up showing random colours). */
void leds_init(void);

/* Each setter updates the chain immediately and calls the change
 * callback. Return false on a bad index. */
bool leds_set_on(uint8_t index, bool on);
bool leds_set_color(uint8_t index, uint8_t r, uint8_t g, uint8_t b); /* also turns it on */
void leds_set_brightness(uint8_t brightness);

/* Snapshot of the current state. */
uint8_t leds_count(void);
uint8_t leds_brightness(void);
led_state_t leds_get(uint8_t index);

/* Called (from whichever task made the change) after every update --
 * ble_link uses it to notify the web page so it follows CLI changes. */
void leds_set_change_cb(void (*cb)(void));

/* Registers the "led" CLI command. */
void leds_register_cli_commands(void);
