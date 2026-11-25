package server

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

// hub wakes up connected WebSocket sessions and sends UnifiedPush pings to
// devices that are offline.
type hub struct {
	mu       sync.Mutex
	conns    map[int64]map[chan struct{}]struct{}
	lastPush map[int64]time.Time
	store    *Store
	client   *http.Client
}

func newHub(s *Store) *hub {
	return &hub{
		conns:    map[int64]map[chan struct{}]struct{}{},
		lastPush: map[int64]time.Time{},
		store:    s,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *hub) subscribe(dev int64) (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if h.conns[dev] == nil {
		h.conns[dev] = map[chan struct{}]struct{}{}
	}
	h.conns[dev][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.conns[dev], ch)
		if len(h.conns[dev]) == 0 {
			delete(h.conns, dev)
		}
		h.mu.Unlock()
	}
}

// kick closes all sessions of a device (after kill/delete).
func (h *hub) kick(dev int64) {
	h.mu.Lock()
	for ch := range h.conns[dev] {
		close(ch)
	}
	delete(h.conns, dev)
	h.mu.Unlock()
}

// notify wakes the device's sessions, or pushes if none are connected.
// Pushes are coalesced to at most one per 10 s per device.
func (h *hub) notify(dev int64) {
	h.mu.Lock()
	online := len(h.conns[dev]) > 0
	for ch := range h.conns[dev] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	push := !online && time.Since(h.lastPush[dev]) > 10*time.Second
	if push {
		h.lastPush[dev] = time.Now()
	}
	h.mu.Unlock()
	if push {
		go h.push(dev)
	}
}

func (h *hub) push(dev int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u, err := h.store.PushURL(ctx, dev)
	if err != nil || u == "" {
		return
	}
	// Empty body: the push carries no information, it only wakes the app.
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(nil))
	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("push failed: %v", err)
		return
	}
	resp.Body.Close()
}
