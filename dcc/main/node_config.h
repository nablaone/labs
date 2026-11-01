#pragma once

/*
 * Compile-time configuration for the ESP32-C3 Super Mini board: pin
 * assignments and firmware version.
 *
 * Super Mini pin notes: GPIO0-10 and GPIO20/21 are broken out. GPIO2,
 * GPIO8 and GPIO9 are strapping pins (sampled at reset); GPIO9 is the
 * BOOT button; GPIO18/19 are the USB D-/D+ lines (not broken out --
 * don't use them, the console runs over USB).
 */

#include "driver/gpio.h"

/* __DATE__ is the compiler builtin build date ("Sep  2 2026", note space-
 * padded single-digit days) -- auto-updates every build so it can't go
 * stale the way a hand-maintained date would (the "version" CLI command
 * shows this). */
#define FIRMWARE_VERSION "0.1.0 (" __DATE__ ")"

/*
 * Onboard blue LED on GPIO8, wired between 3V3 and the pin -- so it's
 * active-low: driving the pin 0 lights it. LED_ON_LEVEL captures that so
 * callers think in on/off rather than pin levels. Confirmed on real
 * hardware (2026-11-01).
 */
#define LED_GPIO     GPIO_NUM_8
#define LED_ON_LEVEL 0

/*
 * DCC track signal input from the 6N137 optocoupler's open-drain
 * output (pulled up to 3V3 externally) -- see
 * ../docs/dcc-accessory-primer.md section 6. GPIO4: free, not a strapping
 * pin, not USB. Low while the opto's input LED conducts, i.e. during one
 * half of every DCC bit.
 */
#define DCC_IN_GPIO GPIO_NUM_4
