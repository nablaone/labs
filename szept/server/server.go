// Package server implements the szept v1 mailbox server (PROTOCOL.md §8).
package server

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"szept/proto"
)

type Config struct {
	DataDir    string   // holds szept.db and blobs/
	PublicURL  string   // base URL clients use, e.g. https://szept.example.lan
	PushAllow  []string // allowed push URL prefixes; empty = any https URL
	TrustProxy bool     // use X-Forwarded-For for rate limiting
}

type Server struct {
	cfg   Config
	store *Store
	hub   *hub
	mux   *http.ServeMux

	limSend, limIP, limKill, limReg *limiter
}

func New(cfg Config) (*Server, error) {
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "blobs"), 0o700); err != nil {
		return nil, err
	}
	st, err := OpenStore(filepath.Join(cfg.DataDir, "szept.db"))
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:     cfg,
		store:   st,
		hub:     newHub(st),
		mux:     http.NewServeMux(),
		limSend: newLimiter(120, time.Minute), // per send token
		limIP:   newLimiter(600, time.Minute), // per IP, all sends
		limKill: newLimiter(5, time.Hour),     // per IP
		limReg:  newLimiter(10, time.Hour),    // per IP, registration
	}
	s.routes()
	return s, nil
}

func (s *Server) Store() *Store                                    { return s.store }
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }
func (s *Server) Close() error                                     { return s.store.Close() }

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("POST /v1/devices", s.createDevice)
	m.HandleFunc("PATCH /v1/devices/me", s.auth(s.patchDevice))
	m.HandleFunc("DELETE /v1/devices/me", s.auth(s.deleteDevice))
	m.HandleFunc("POST /v1/invites", s.auth(s.createInvite))
	m.HandleFunc("PUT /v1/mailboxes/{id}", s.auth(s.createMailbox))
	m.HandleFunc("DELETE /v1/mailboxes/{id}", s.auth(s.deleteMailbox))
	m.HandleFunc("POST /v1/mailboxes/{id}/messages", s.send)
	m.HandleFunc("GET /v1/ws", s.auth(s.ws))
	m.HandleFunc("GET /v1/inbox", s.auth(s.inbox))
	m.HandleFunc("POST /v1/inbox/ack", s.auth(s.ack))
	m.HandleFunc("POST /v1/blobs", s.auth(s.uploadBlob))
	m.HandleFunc("GET /v1/blobs/{id}", s.downloadBlob)
	m.HandleFunc("POST /v1/kill", s.kill)
	m.HandleFunc("GET /v1/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
}

// ---- helpers ----

var b64 = base64.RawURLEncoding

// b64bytes is []byte encoded as unpadded base64url in JSON.
type b64bytes []byte

func (b b64bytes) MarshalJSON() ([]byte, error) { return json.Marshal(b64.EncodeToString(b)) }
func (b *b64bytes) UnmarshalJSON(d []byte) error {
	var s string
	if err := json.Unmarshal(d, &s); err != nil {
		return err
	}
	v, err := b64.DecodeString(s)
	*b = v
	return err
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func storeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		fail(w, 404, "not found")
	case errors.Is(err, ErrConflict):
		fail(w, 409, "exists")
	case errors.Is(err, ErrForbidden):
		fail(w, 403, "forbidden")
	case errors.Is(err, ErrLimit):
		fail(w, 429, "limit reached")
	default:
		log.Printf("store error: %v", err)
		fail(w, 500, "internal error")
	}
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(v)
}

// token parses "Authorization: <scheme> <base64url>".
func token(r *http.Request, scheme string) []byte {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, scheme+" ") {
		return nil
	}
	t, err := b64.DecodeString(strings.TrimSpace(h[len(scheme)+1:]))
	if err != nil || len(t) != 32 {
		return nil
	}
	return t
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if f := r.Header.Get("X-Forwarded-For"); f != "" {
			return strings.TrimSpace(strings.Split(f, ",")[0])
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

type authed func(w http.ResponseWriter, r *http.Request, dev int64)

func (s *Server) auth(h authed) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := token(r, "Bearer")
		if t == nil {
			fail(w, 401, "unauthorized")
			return
		}
		dev, err := s.store.Device(r.Context(), t)
		if err != nil {
			fail(w, 401, "unauthorized")
			return
		}
		h(w, r, dev)
	}
}

func pathID(r *http.Request, n int) []byte {
	id, err := hex.DecodeString(r.PathValue("id"))
	if err != nil || len(id) != n {
		return nil
	}
	return id
}

func (s *Server) validPushURL(u string) bool {
	if u == "" {
		return true
	}
	if len(u) > 512 || !strings.HasPrefix(u, "https://") {
		return false
	}
	if len(s.cfg.PushAllow) == 0 {
		return true
	}
	for _, p := range s.cfg.PushAllow {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return false
}

// ---- devices & invites ----

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	if !s.limReg.allow(s.clientIP(r)) {
		fail(w, 429, "rate limited")
		return
	}
	var req struct {
		Invite    b64bytes `json:"invite"`
		TokenHash b64bytes `json:"token_hash"`
		KillHash  b64bytes `json:"kill_hash"`
		PushURL   string   `json:"push_url"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Invite) != 16 || len(req.TokenHash) != 32 ||
		(req.KillHash != nil && len(req.KillHash) != 32) {
		fail(w, 400, "bad request")
		return
	}
	if !s.validPushURL(req.PushURL) {
		fail(w, 400, "push_url not allowed")
		return
	}
	if err := s.store.CreateDevice(r.Context(), req.Invite, req.TokenHash, req.KillHash, req.PushURL); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(201)
}

func (s *Server) patchDevice(w http.ResponseWriter, r *http.Request, dev int64) {
	var req struct {
		PushURL string `json:"push_url"`
	}
	if err := readJSON(r, &req); err != nil || !s.validPushURL(req.PushURL) {
		fail(w, 400, "bad request")
		return
	}
	if err := s.store.SetPushURL(r.Context(), dev, req.PushURL); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) removeDevice(ctx context.Context, dev int64) error {
	blobs, err := s.store.DeleteDevice(ctx, dev)
	if err != nil {
		return err
	}
	for _, id := range blobs {
		os.Remove(s.blobPath(id))
	}
	s.hub.kick(dev)
	return nil
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request, dev int64) {
	if err := s.removeDevice(r.Context(), dev); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request, dev int64) {
	inv, exp, err := s.store.CreateInvite(r.Context(), dev, proto.MaxOpenInvites, proto.InviteTTL)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"invite": b64bytes(inv), "expires": exp.UnixMilli()})
}

// ---- mailboxes & messages ----

func (s *Server) createMailbox(w http.ResponseWriter, r *http.Request, dev int64) {
	id := pathID(r, 16)
	var req struct {
		SendHash b64bytes `json:"send_hash"`
	}
	if id == nil || readJSON(r, &req) != nil || len(req.SendHash) != 32 {
		fail(w, 400, "bad request")
		return
	}
	if err := s.store.CreateMailbox(r.Context(), dev, id, req.SendHash); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(201)
}

func (s *Server) deleteMailbox(w http.ResponseWriter, r *http.Request, dev int64) {
	id := pathID(r, 16)
	if id == nil {
		fail(w, 400, "bad request")
		return
	}
	if err := s.store.DeleteMailbox(r.Context(), dev, id); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, 16)
	t := token(r, "Send")
	if id == nil || t == nil {
		fail(w, 400, "bad request")
		return
	}
	th := hash(t)
	if !s.limIP.allow(s.clientIP(r)) || !s.limSend.allow(string(th)) {
		fail(w, 429, "rate limited")
		return
	}
	dev, sendHash, err := s.store.Mailbox(r.Context(), id)
	if err != nil {
		storeErr(w, err)
		return
	}
	if subtle.ConstantTimeCompare(th, sendHash) != 1 {
		fail(w, 403, "forbidden")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, proto.MaxEnvelope+1))
	if err != nil {
		fail(w, 400, "bad request")
		return
	}
	if len(body) > proto.MaxEnvelope {
		fail(w, 413, "too large")
		return
	}
	if len(body) == 0 {
		fail(w, 400, "empty")
		return
	}
	mid, err := s.store.Enqueue(r.Context(), dev, id, body)
	if err != nil {
		storeErr(w, err)
		return
	}
	s.hub.notify(dev)
	writeJSON(w, 202, map[string]any{"id": b64bytes(mid)})
}

type wireMsg struct {
	Type    string   `json:"type"`
	Mailbox b64bytes `json:"mailbox"`
	ID      b64bytes `json:"id"`
	TS      int64    `json:"ts"`
	Body    b64bytes `json:"body"`
}

func toWire(m Message) wireMsg {
	return wireMsg{Type: "msg", Mailbox: m.Mailbox, ID: m.ID, TS: m.TS, Body: m.Body}
}

func (s *Server) inbox(w http.ResponseWriter, r *http.Request, dev int64) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	msgs, err := s.store.Pending(r.Context(), dev, 0, limit)
	if err != nil {
		storeErr(w, err)
		return
	}
	out := make([]wireMsg, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toWire(m))
	}
	writeJSON(w, 200, map[string]any{"msgs": out})
}

type ackReq struct {
	Type string     `json:"type"`
	IDs  []b64bytes `json:"ids"`
}

func (s *Server) doAck(ctx context.Context, dev int64, a ackReq) error {
	ids := make([][]byte, 0, len(a.IDs))
	for _, id := range a.IDs {
		if len(id) == 16 {
			ids = append(ids, id)
		}
	}
	return s.store.Ack(ctx, dev, ids)
}

func (s *Server) ack(w http.ResponseWriter, r *http.Request, dev int64) {
	var a ackReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&a); err != nil || len(a.IDs) > 1000 {
		fail(w, 400, "bad request")
		return
	}
	if err := s.doAck(r.Context(), dev, a); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request, dev int64) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(64 << 10)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	wake, unsub := s.hub.subscribe(dev)
	defer unsub()

	// Reader: acks from the client.
	go func() {
		defer cancel()
		for {
			var a ackReq
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if json.Unmarshal(data, &a) == nil && a.Type == "ack" && len(a.IDs) <= 1000 {
				if err := s.doAck(ctx, dev, a); err != nil {
					log.Printf("ack: %v", err)
				}
			}
		}
	}()

	var last int64
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		// Drain everything queued after the last message we sent.
		for {
			msgs, err := s.store.Pending(ctx, dev, last, 100)
			if err != nil {
				return
			}
			for _, m := range msgs {
				b, _ := json.Marshal(toWire(m))
				if err := c.Write(ctx, websocket.MessageText, b); err != nil {
					return
				}
				last = m.Seq
			}
			if len(msgs) < 100 {
				break
			}
		}
		select {
		case <-ctx.Done():
			c.Close(websocket.StatusNormalClosure, "")
			return
		case _, ok := <-wake:
			if !ok { // device deleted
				c.Close(websocket.StatusPolicyViolation, "device removed")
				return
			}
		case <-ping.C:
			pctx, pc := context.WithTimeout(ctx, 15*time.Second)
			err := c.Ping(pctx)
			pc()
			if err != nil {
				return
			}
		}
	}
}

// ---- blobs ----

func (s *Server) blobPath(id []byte) string {
	return filepath.Join(s.cfg.DataDir, "blobs", hex.EncodeToString(id))
}

func (s *Server) uploadBlob(w http.ResponseWriter, r *http.Request, dev int64) {
	maxDl := 1
	if v := r.Header.Get("X-Max-Downloads"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1000 {
			fail(w, 400, "bad X-Max-Downloads")
			return
		}
		maxDl = n
	}
	id := randBytes(16)
	tmp, err := os.CreateTemp(filepath.Join(s.cfg.DataDir, "blobs"), "up-*")
	if err != nil {
		storeErr(w, err)
		return
	}
	defer os.Remove(tmp.Name()) // no-op after successful rename
	n, err := io.Copy(tmp, io.LimitReader(r.Body, proto.MaxBlob+1))
	tmp.Close()
	if err != nil {
		fail(w, 400, "upload failed")
		return
	}
	if n > proto.MaxBlob {
		fail(w, 413, "too large")
		return
	}
	if err := os.Rename(tmp.Name(), s.blobPath(id)); err != nil {
		storeErr(w, err)
		return
	}
	if err := s.store.CreateBlob(r.Context(), dev, id, maxDl, proto.BlobTTL); err != nil {
		os.Remove(s.blobPath(id))
		storeErr(w, err)
		return
	}
	base := s.cfg.PublicURL
	if base == "" {
		base = "https://" + r.Host
	}
	writeJSON(w, 201, map[string]string{"url": strings.TrimRight(base, "/") + "/v1/blobs/" + hex.EncodeToString(id)})
}

func (s *Server) downloadBlob(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, 16)
	if id == nil {
		fail(w, 404, "not found")
		return
	}
	if ok, err := s.store.BlobExists(r.Context(), id); err != nil || !ok {
		fail(w, 404, "not found")
		return
	}
	f, err := os.Open(s.blobPath(id))
	if err != nil {
		fail(w, 404, "not found")
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	if n, err := io.Copy(w, f); err != nil || n != st.Size() {
		return // incomplete: don't count it, the client may retry
	}
	if done, err := s.store.BlobDownloaded(context.WithoutCancel(r.Context()), id); err == nil && done {
		os.Remove(s.blobPath(id)) // open handle keeps working on unix
	}
}

// ---- kill ----

func (s *Server) kill(w http.ResponseWriter, r *http.Request) {
	defer w.WriteHeader(204) // always the same answer
	if !s.limKill.allow(s.clientIP(r)) {
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if readJSON(r, &req) != nil {
		return
	}
	h, err := proto.KillHash(req.Code)
	if err != nil {
		return
	}
	dev, err := s.store.DeviceByKill(r.Context(), h)
	if err != nil {
		return
	}
	if err := s.removeDevice(r.Context(), dev); err != nil {
		log.Printf("kill: %v", err)
	}
}

// ---- maintenance ----

// Janitor expires old data every interval until ctx is done. On start it also
// removes blob files that have no database record.
func (s *Server) Janitor(ctx context.Context, every time.Duration) {
	s.cleanOrphans(ctx)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		ids, err := s.store.Expire(ctx, proto.MessageTTL)
		if err != nil {
			log.Printf("expire: %v", err)
		}
		for _, id := range ids {
			os.Remove(s.blobPath(id))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Server) cleanOrphans(ctx context.Context) {
	ids, err := s.store.AllBlobIDs(ctx)
	if err != nil {
		return
	}
	live := map[string]bool{}
	for _, id := range ids {
		live[hex.EncodeToString(id)] = true
	}
	entries, _ := os.ReadDir(filepath.Join(s.cfg.DataDir, "blobs"))
	for _, e := range entries {
		if !live[e.Name()] {
			os.Remove(filepath.Join(s.cfg.DataDir, "blobs", e.Name()))
		}
	}
}
