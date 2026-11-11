#pragma once

#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

void heartbeat_task_init(void);
void heartbeat_task(void *arg);

/* Fast blink (detector sees a hand) vs the normal slow "alive" blink. */
void heartbeat_set_fast(bool fast);

#ifdef __cplusplus
}
#endif
