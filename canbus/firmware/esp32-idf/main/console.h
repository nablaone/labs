#pragma once

/* esp_console/linenoise setup plus the built-in "help"/"version"/"exit"
 * commands, common to every node -- this is "the same debug strategy"
 * every node's firmware shares. Other modules register their own
 * commands (state's "counter", heartbeat_task's "rate", can's "can")
 * after this returns. */
void console_init(void);

/* Waits for Enter on the console UART while logs flow normally; switches
 * into an interactive CLI (silencing logs) once it sees one, then goes
 * back to waiting once the user types "exit". */
void console_task(void *arg);
