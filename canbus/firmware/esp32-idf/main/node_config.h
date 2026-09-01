#pragma once

/*
 * Per-node compile-time configuration: which task/hardware modules this
 * build includes, plus their pin assignments. This is the file a new node
 * (motor/controller/panel, see ../../../docs/project-charter.md) copies
 * and edits -- same task modules, same CLI/logging pattern as every other
 * node, just a different pin set and a different subset enabled.
 *
 * Runtime (NVRAM-backed) reconfiguration is a planned extension, not
 * implemented yet -- these are compile-time-only for now.
 */

#include "driver/gpio.h"

#define NODE_ENABLE_LED       1
#define NODE_ENABLE_HEARTBEAT 1
#define NODE_ENABLE_BUTTON    1
#define NODE_ENABLE_DISPLAY   1
#define NODE_ENABLE_CAN       1

#define FIRMWARE_VERSION "0.2.0"

/*
 * LED on GPIO2 (onboard LED) and button on GPIO0 (onboard BOOT button) --
 * both onboard, no breadboard wiring needed. Both are strapping pins
 * (sampled at boot to select flash/boot mode), which is why the earlier
 * Zephyr app avoided them in favor of an external button on GPIO33 -- but
 * once the app is running, GPIO0 reads like any other input (it's only
 * sampled at reset), and the onboard LED's light loading on GPIO2 doesn't
 * disturb boot-mode sensing in practice (confirmed on real hardware). The
 * board already has an external pull-up on GPIO0 for its BOOT button;
 * button_task_init()'s gpio_pullup_en() just reinforces it.
 */
#define LED_GPIO    GPIO_NUM_2
#define BUTTON_GPIO GPIO_NUM_0

/*
 * TWAI (CAN) TX/RX to the SN65HVD230 transceiver -- see
 * ../../../docs/can-bus-bringup-plan.md for the full pin-choice reasoning
 * (not strapping, not input-only, not UART0/flash-SPI, free). Silkscreen
 * on this board's DevKit V1-style clone prints these as D21/D22 -- same
 * physical pins as GPIO21/22, just an alias.
 */
#define CAN_TX_GPIO GPIO_NUM_21
#define CAN_RX_GPIO GPIO_NUM_22

#define POLL_MS      100
#define HEARTBEAT_MS 1000
#define DISPLAY_MS   10000
