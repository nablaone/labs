#pragma once

/*
 * HD44780 character LCD over a PCF8574 I2C backpack (16x2). Like can.c,
 * this is "hardware code" other modules reach into directly rather than
 * a self-contained task -- lcd_display() is the public entry point
 * every other module calls (button_task, display_task, etc.) to put
 * text on the physical display without touching I2C themselves.
 */

void lcd_task_init(void);
void lcd_task(void *arg);

/*
 * Requests that line1/line2 (each truncated to 16 chars) be shown on
 * the display. Mutex-protected, non-blocking, and asynchronous: this
 * only updates in-memory state and returns immediately -- the actual
 * I2C write happens later, off lcd_task()'s own loop, so a caller never
 * blocks on the (comparatively slow) bus transaction. A call with text
 * identical to what's already shown is a no-op.
 */
void lcd_display(const char *line1, const char *line2);

/* Registers the "lcd" CLI command (manual bring-up/test of lcd_display()). */
void lcd_task_register_cli_commands(void);
