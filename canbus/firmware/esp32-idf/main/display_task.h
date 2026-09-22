#pragma once

/*
 * Drives a 16x2 HD44780 character LCD over a PCF8574 I2C backpack.
 * Combines what used to be two modules -- lcd_task (the I2C/HD44780
 * driver) and display_task (deciding what to show) -- into one:
 * display_task_init() brings up the physical display; display_task()
 * does nothing else but cycle through every currently registered
 * metrics.c entry, one per DISPLAY_METRIC_MS, key on line 1 / value on
 * line 2. That's the only functionality here -- no CLI command to push
 * arbitrary text, no CAN broadcast, no per-module special-casing. A
 * module that wants something shown just registers a metric (see
 * metrics.h); this task doesn't know or care which module that was.
 */

void display_task_init(void);
void display_task(void *arg);
