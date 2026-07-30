package actionlog

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type EventType string

const (
	// Discovery is a synthetic event type emitted by the poller — not stored in the log.
	EventDiscovery   EventType = "discovery"
	EventRowClick    EventType = "row_click"
	EventButtonClick EventType = "button_click"
	EventInputSubmit EventType = "input_submit"
	EventCellEdit    EventType = "cell_edit"
)

type ActionEvent struct {
	ID        int64          `json:"id"`
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

type Log struct {
	mu       sync.RWMutex
	events   []ActionEvent
	maxSize  int
	counter  int64
	filePath string
}

// LoadOrNew loads events from filePath if it exists, otherwise returns an empty log.
func LoadOrNew(filePath string, maxSize int) *Log {
	if maxSize <= 0 {
		maxSize = 200
	}
	l := &Log{maxSize: maxSize, filePath: filePath}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return l
	}
	var events []ActionEvent
	if err := json.Unmarshal(data, &events); err != nil {
		log.Printf("[actionlog] failed to parse %s: %v — starting fresh", filePath, err)
		return l
	}
	// Respect maxSize, keep the most recent events
	if len(events) > maxSize {
		events = events[len(events)-maxSize:]
	}
	l.events = events
	if len(events) > 0 {
		l.counter = events[len(events)-1].ID
	}
	log.Printf("[actionlog] loaded %d events from %s", len(events), filePath)
	return l
}

func (l *Log) save() {
	if l.filePath == "" {
		return
	}
	data, err := json.MarshalIndent(l.events, "", "  ")
	if err != nil {
		log.Printf("[actionlog] marshal error: %v", err)
		return
	}
	if err := os.WriteFile(l.filePath, data, 0644); err != nil {
		log.Printf("[actionlog] write error: %v", err)
	}
}

func (l *Log) Append(typ EventType, payload map[string]any) ActionEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counter++
	ev := ActionEvent{
		ID:        l.counter,
		Type:      typ,
		Timestamp: time.Now(),
		Payload:   payload,
	}
	l.events = append(l.events, ev)
	if len(l.events) > l.maxSize {
		l.events = l.events[len(l.events)-l.maxSize:]
	}
	l.save()
	return ev
}

func (l *Log) Last(n int) []ActionEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n <= 0 || n >= len(l.events) {
		out := make([]ActionEvent, len(l.events))
		copy(out, l.events)
		return out
	}
	out := make([]ActionEvent, n)
	copy(out, l.events[len(l.events)-n:])
	return out
}
