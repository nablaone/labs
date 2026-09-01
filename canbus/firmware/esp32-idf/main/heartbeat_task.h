#pragma once

void heartbeat_task_init(void);
void heartbeat_task(void *arg);

/* Registers the "rate" CLI command. */
void heartbeat_task_register_cli_commands(void);
