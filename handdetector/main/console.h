#pragma once

/* esp_console/linenoise setup plus the built-in "help"/"version"/"exit"
 * commands. Other modules register their own commands after this
 * returns. */
void console_init(void);

/* Waits for Enter on the USB console while logs flow normally; switches
 * into an interactive CLI (silencing logs) once it sees one, then goes
 * back to waiting once the user types "exit". */
void console_task(void *arg);
