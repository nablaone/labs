package agent

import (
	"context"
	"log"
	"time"

	"autoapp/internal/actionlog"
	"autoapp/internal/db"
	"autoapp/internal/discovery"
)

// Poller periodically reads pg_stat_statements and triggers the agent when
// new query patterns are found.
type Poller struct {
	debouncer *Debouncer
	exec      *db.Executor
	disc      *discovery.State
	interval  time.Duration
}

func NewPoller(d *Debouncer, exec *db.Executor, disc *discovery.State, interval time.Duration) *Poller {
	return &Poller{
		debouncer: d,
		exec:      exec,
		disc:      disc,
		interval:  interval,
	}
}

func (p *Poller) Start(ctx context.Context) {
	go func() {
		p.check(ctx)

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.check(ctx)
			}
		}
	}()
}

func (p *Poller) check(ctx context.Context) {
	stmts, err := p.exec.GetStatements(ctx)
	if err != nil {
		log.Printf("[poller] pg_stat_statements error: %v", err)
		return
	}

	var newStmts []db.StatementInfo
	for _, s := range stmts {
		if !p.disc.IsSeen(s.QueryID) {
			newStmts = append(newStmts, s)
		}
	}

	if len(newStmts) == 0 {
		return
	}

	log.Printf("[poller] %d new query pattern(s) found", len(newStmts))

	for _, s := range newStmts {
		p.disc.MarkSeen(s.QueryID, s.Query, s.Calls)
	}

	p.debouncer.Trigger(actionlog.ActionEvent{
		Type:    actionlog.EventDiscovery,
		Payload: map[string]any{"statements": newStmts},
	})
}
