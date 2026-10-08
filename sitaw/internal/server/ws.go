package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"sitaw/internal/store"
)

// Wire protocol: JSON text frames, discriminated by "t". Everything a client
// sends or receives is scoped to its own team.
//
// client -> server
//
//	{"t":"pos",  "pos": {lat, lon, acc, hdg, spd, ts}}
//	{"t":"put",  "item": Item}            upsert, or tombstone with deleted:true
//
// server -> client
//
//	{"t":"snapshot", "now": ms, "you": User, "team": Team, "users": [...], "items": [...], "positions": [...]}
//	{"t":"pos",   "pos": Position}
//	{"t":"item",  "item": Item}
//	{"t":"leave", "userId": "..."}       user was removed; drop their marker
//	{"t":"user",  "user": PublicUser}    someone joined the team
//	{"t":"ack",   "id": "...", "ok": bool, "error": "...", "item": Item, "sentAt": ms}
//
// ack: ok=false with item set means the server copy won (LWW); sentAt echoes
// the client's updatedAt for outbox bookkeeping. "now" in the snapshot lets
// clients correct their clock offset before stamping edits.
type msg struct {
	T         string             `json:"t"`
	Pos       *store.Position    `json:"pos,omitempty"`
	Item      *store.Item        `json:"item,omitempty"`
	ID        string             `json:"id,omitempty"`
	OK        *bool              `json:"ok,omitempty"`
	Error     string             `json:"error,omitempty"`
	You       *store.PublicUser  `json:"you,omitempty"`
	Team      *store.Team        `json:"team,omitempty"`
	Users     []store.PublicUser `json:"users,omitempty"`
	Items     []store.Item       `json:"items,omitempty"`
	Positions []store.Position   `json:"positions,omitempty"`
	SentAt    int64              `json:"sentAt,omitempty"`
	Now       int64              `json:"now,omitempty"`
	UserID    string             `json:"userId,omitempty"`
	User      *store.PublicUser  `json:"user,omitempty"`
}

// Client clocks may be off; never let an edit claim to be from the far future,
// or it would win every later LWW comparison.
const maxClockSkew = 5 * time.Minute

type client struct {
	user  store.User
	send  chan []byte
	close func() // ends the connection
}

type Hub struct {
	log     *slog.Logger
	mu      sync.Mutex
	clients map[*client]struct{}
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{log: log, clients: map[*client]struct{}{}}
}

// join registers c and queues its snapshot under the hub lock, so every
// change is either in the snapshot or broadcast to c afterwards.
func (h *Hub) join(c *client, snapshot func() ([]byte, error)) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, err := snapshot()
	if err != nil {
		return err
	}
	c.send <- b
	h.clients[c] = struct{}{}
	return nil
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// kick closes every connection of the given user.
func (h *Hub) kick(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if c.user.ID == userID {
			c.close()
		}
	}
}

// broadcast sends m to every connected member of teamID. A client whose
// buffer is full is dropped; it gets a fresh snapshot when it reconnects.
func (h *Hub) broadcast(teamID string, m msg) {
	b, err := json.Marshal(m)
	if err != nil {
		h.log.Error("marshal", "err", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if c.user.TeamID != teamID {
			continue
		}
		select {
		case c.send <- b:
		default:
			delete(h.clients, c)
			close(c.send)
		}
	}
}

func (s *Server) snapshot(u store.User) ([]byte, error) {
	team, err := s.store.Team(u.TeamID)
	if err != nil {
		return nil, err
	}
	users, err := s.store.Users(u.TeamID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.Items(u.TeamID)
	if err != nil {
		return nil, err
	}
	positions, err := s.store.Positions(u.TeamID)
	if err != nil {
		return nil, err
	}
	pub := u.Public()
	return json.Marshal(msg{
		T: "snapshot", Now: time.Now().UnixMilli(), You: &pub, Team: &team,
		Users: users, Items: items, Positions: positions,
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Browsers can't set headers on a WebSocket, so the token rides in the query.
	u, err := s.store.UserByToken(r.URL.Query().Get("token"))
	if errors.Is(err, store.ErrUnauthorized) {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &client{user: u, send: make(chan []byte, 256), close: cancel}
	if err := s.hub.join(c, func() ([]byte, error) { return s.snapshot(u) }); err != nil {
		s.log.Error("snapshot", "err", err)
		conn.Close(websocket.StatusInternalError, "snapshot failed")
		return
	}
	defer s.hub.remove(c)
	s.log.Info("ws connected", "user", u.Callsign, "team", u.TeamID)

	go s.writeLoop(ctx, cancel, conn, c)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			s.log.Info("ws disconnected", "user", u.Callsign, "err", err)
			return
		}
		var m msg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		s.handleMsg(c, m)
	}
}

func (s *Server) writeLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, c *client) {
	defer cancel()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b, ok := <-c.send:
			if !ok {
				conn.Close(websocket.StatusPolicyViolation, "too slow")
				return
			}
			wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, b)
			wcancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) handleMsg(c *client, m msg) {
	now := time.Now()
	team := c.user.TeamID
	switch m.T {
	case "pos":
		p := m.Pos
		if p == nil || !validLatLon(p.Lat, p.Lon) {
			return
		}
		p.UserID, p.Callsign = c.user.ID, c.user.Callsign
		if p.TS <= 0 || p.TS > now.Add(maxClockSkew).UnixMilli() {
			p.TS = now.UnixMilli()
		}
		stored, err := s.store.SetPosition(c.user, *p)
		if err != nil {
			s.log.Error("set position", "err", err)
			return
		}
		if stored {
			s.hub.broadcast(team, msg{T: "pos", Pos: p})
		}

	case "put":
		it := m.Item
		if it == nil {
			return
		}
		if err := validateItem(it); err != nil {
			s.reply(c, msg{T: "ack", ID: it.ID, OK: ptr(false), Error: err.Error()})
			return
		}
		sentAt := it.UpdatedAt
		if it.UpdatedAt > now.Add(maxClockSkew).UnixMilli() {
			it.UpdatedAt = now.UnixMilli()
		}
		stored, applied, err := s.store.PutItem(team, *it, c.user)
		if errors.Is(err, store.ErrForeignItem) {
			s.reply(c, msg{T: "ack", ID: it.ID, OK: ptr(false), Error: "id in use"})
			return
		}
		if errors.Is(err, store.ErrFolderInUse) {
			// Hand back the folder so the client can undo its local delete.
			s.reply(c, msg{T: "ack", ID: it.ID, OK: ptr(false), Error: "only empty, non-personal folders can be deleted", Item: &stored, SentAt: sentAt})
			return
		}
		if err != nil {
			s.log.Error("put item", "err", err)
			return // stays in the client's outbox and is retried on reconnect
		}
		s.reply(c, msg{T: "ack", ID: it.ID, OK: ptr(applied), Item: &stored, SentAt: sentAt})
		if applied {
			s.hub.broadcast(team, msg{T: "item", Item: &stored})
		}
	}
}

// reply queues m for one client only.
func (s *Server) reply(c *client, m msg) {
	b, _ := json.Marshal(m)
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	if _, ok := s.hub.clients[c]; !ok {
		return
	}
	select {
	case c.send <- b:
	default:
	}
}

func ptr[T any](v T) *T { return &v }
