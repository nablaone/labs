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

#define NODE_ENABLE_HEARTBEAT 1
#define NODE_ENABLE_CAN       1
/* Requires NODE_ENABLE_CAN -- see ping_role.c/pong_role.c. */
#define NODE_ENABLE_PINGPONG  1

/* __DATE__ is the compiler builtin build date ("Sep  2 2026", note space-
 * padded single-digit days) -- auto-updates every build so it can't go
 * stale the way a hand-maintained date would, useful for telling boards
 * apart during bring-up when it's not obvious which one has the latest
 * flash (the "version" CLI command and the "version" metric -- see
 * metrics.c -- both show this). */
#define FIRMWARE_VERSION "0.2.0 (" __DATE__ ")"

/*
 * LED on GPIO2 (onboard LED, driven by heartbeat_task as a visual pulse)
 * -- onboard, no breadboard wiring needed. A strapping pin (sampled at
 * boot to select flash/boot mode), but its light loading doesn't disturb
 * boot-mode sensing in practice (confirmed on real hardware).
 *
 * GPIO0 (onboard BOOT button) is unused by firmware now that button_task
 * is gone -- free for something else later.
 */
#define LED_GPIO GPIO_NUM_2

/*
 * TWAI (CAN) TX/RX to the SN65HVD230 transceiver -- see
 * ../../../docs/can-bus-bringup-plan.md for the full pin-choice reasoning
 * (not strapping, not input-only, not UART0/flash-SPI, free). Silkscreen
 * on this board's DevKit V1-style clone prints these as D21/D22 -- same
 * physical pins as GPIO21/22, just an alias.
 */
#define CAN_TX_GPIO GPIO_NUM_21
#define CAN_RX_GPIO GPIO_NUM_22

/*
 * I2C for the HD44780/PCF8574 character LCD -- free, non-strapping pins
 * (GPIO21/22 are already CAN, GPIO0/2 are the LED/button strapping
 * pins). LCD_I2C_ADDR is the common PCF8574 backpack default; some ship
 * at 0x3F -- verify/adjust for your actual board during bring-up.
 *
 * Originally tried on GPIO32/33 (also free/non-strapping); moved to
 * GPIO26/27 during bring-up while chasing a "no response at all" bus
 * result. That turned out to be a red herring -- GPIO26/27 showed the
 * exact same symptom, and the real cause (a marginal bus needing
 * retries -- see display_task.c's pcf8574_write()) was pin-independent.
 * Left on 26/27 since that's what ended up wired/verified; no
 * functional reason to move back.
 */
#define I2C_SDA_GPIO GPIO_NUM_26
#define I2C_SCL_GPIO GPIO_NUM_27
#define LCD_I2C_ADDR 0x27

/* display_task: how long each registered metric stays on screen before
 * cycling to the next one. See metrics.c/display_task.c. */
#define DISPLAY_METRIC_MS 1000

/* ping_role.c: PING_PERIOD_MS is ping_task's period between pings;
 * PING_TIMEOUT_MS is how long ping_task waits for the matching PONG
 * before logging a timeout. pong_role.c's pong_task blocks indefinitely
 * instead of polling, so it doesn't use either. */
#define PING_PERIOD_MS  1000
#define PING_TIMEOUT_MS 300
