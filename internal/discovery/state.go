package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// QueryRecord tracks a query pattern discovered in pg_stat_statements.
type QueryRecord struct {
	Query            string    `json:"query"`
	CallsAtDiscovery int64     `json:"calls_at_discovery"`
	FirstSeen        time.Time `json:"first_seen"`
}

type stateFile struct {
	Queries map[string]QueryRecord `json:"queries"`
}

// State persists which pg_stat_statements queryids have already been processed.
type State struct {
	mu       sync.Mutex
	queries  map[string]QueryRecord
	filePath string
}

func LoadOrNew(filePath string) *State {
	s := &State{
		queries:  make(map[string]QueryRecord),
		filePath: filePath,
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return s
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		log.Printf("[discovery] failed to parse %s: %v — starting fresh", filePath, err)
		return s
	}
	if sf.Queries != nil {
		s.queries = sf.Queries
	}
	log.Printf("[discovery] loaded %d known query patterns from %s", len(s.queries), filePath)
	return s
}

func key(queryID int64) string { return fmt.Sprintf("%d", queryID) }

// IsSeen returns true if this queryid has already been processed.
func (s *State) IsSeen(queryID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.queries[key(queryID)]
	return ok
}

// MarkSeen records a queryid as processed.
func (s *State) MarkSeen(queryID int64, query string, calls int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key(queryID)
	if _, exists := s.queries[k]; !exists {
		s.queries[k] = QueryRecord{
			Query:            query,
			CallsAtDiscovery: calls,
			FirstSeen:        time.Now(),
		}
		s.save()
	}
}

// KnownCount returns the number of known query patterns.
func (s *State) KnownCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queries)
}

func (s *State) save() {
	sf := stateFile{Queries: s.queries}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		log.Printf("[discovery] write error: %v", err)
	}
}
