package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"autoapp/internal/actionlog"
)

// Debouncer ensures at most one agent run is in flight at a time.
// A new trigger within debounceMs cancels any pending run and restarts the timer.
type Debouncer struct {
	agent      *Agent
	debounceMs time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	timer  *time.Timer
}

func NewDebouncer(a *Agent, debounceMs int) *Debouncer {
	if debounceMs <= 0 {
		debounceMs = 300
	}
	return &Debouncer{
		agent:      a,
		debounceMs: time.Duration(debounceMs) * time.Millisecond,
	}
}

// Trigger schedules an agent run for the given event, cancelling any pending run.
func (d *Debouncer) Trigger(ev actionlog.ActionEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel in-flight or pending run
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	if d.timer != nil {
		d.timer.Stop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	d.timer = time.AfterFunc(d.debounceMs, func() {
		d.mu.Lock()
		d.timer = nil
		d.mu.Unlock()

		if err := d.agent.Run(ctx, ev); err != nil {
			if ctx.Err() == nil {
				log.Printf("[agent] run error: %v", err)
			}
		}
	})
}
