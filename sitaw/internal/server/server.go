// Package server exposes the HTTP API, the WebSocket sync channel and the
// embedded web UI. Every authenticated request is scoped to the caller's team.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"sitaw/internal/store"
)

type Server struct {
	store      *store.Store
	hub        *Hub
	static     fs.FS
	adminToken string
	baseURL    string
	log        *slog.Logger
	limiter    *rateLimiter
}

func New(st *store.Store, static fs.FS, adminToken, baseURL string, log *slog.Logger) *Server {
	return &Server{
		store:      st,
		hub:        NewHub(log),
		static:     static,
		adminToken: adminToken,
		baseURL:    strings.TrimRight(baseURL, "/"),
		log:        log,
		limiter:    &rateLimiter{wins: map[string]*window{}},
	}
}

func (s *Server) InviteURL(token string) string {
	return s.baseURL + "/join?t=" + token
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/invites/{token}", s.handleInviteInfo)
	mux.HandleFunc("POST /api/join", s.handleJoin)
	mux.HandleFunc("GET /api/me", s.withUser(s.handleMe))
	mux.HandleFunc("GET /ws", s.handleWS)

	// Agents: managed by people, used by AI agents through /api/v1.
	mux.HandleFunc("GET /api/agents", s.humanOnly(s.handleListAgents))
	mux.HandleFunc("POST /api/agents", s.humanOnly(s.handleCreateAgent))
	mux.HandleFunc("DELETE /api/agents/{id}", s.humanOnly(s.handleRevokeAgent))
	mux.HandleFunc("GET /agent", s.withAPIUser(s.handleAgentInstructions))

	api := func(pattern string, h func(http.ResponseWriter, *http.Request, store.User)) {
		mux.HandleFunc(pattern, s.withAPIUser(h))
	}
	api("GET /api/v1/me", s.handleAPIMe)
	api("GET /api/v1/team", s.handleAPITeam)
	api("GET /api/v1/positions", s.handleAPIPositions)
	api("GET /api/v1/folders", s.handleAPIFolders)
	api("GET /api/v1/objects", s.handleAPIObjects)
	api("GET /api/v1/objects/{id}", s.handleAPIObject)
	api("POST /api/v1/objects", s.handleAPICreate)
	api("PATCH /api/v1/objects/{id}", s.handleAPIUpdate)
	api("DELETE /api/v1/objects/{id}", s.handleAPIDelete)
	api("GET /api/v1/geo/convert", s.handleGeoConvert)
	api("GET /api/v1/geo/measure", s.handleGeoMeasure)
	api("GET /api/v1/geo/nearby", s.handleGeoNearby)
	api("GET /api/v1/geo/inside", s.handleGeoInside)

	mux.HandleFunc("GET /api/admin/teams", s.withAdmin(s.handleListTeams))
	mux.HandleFunc("POST /api/admin/teams", s.withAdmin(s.handleCreateTeam))
	mux.HandleFunc("POST /api/admin/teams/{id}/invites", s.withAdmin(s.handleCreateInvite))
	mux.HandleFunc("GET /api/admin/teams/{id}/users", s.withAdmin(s.handleListUsers))
	mux.HandleFunc("DELETE /api/admin/invites/{token}", s.withAdmin(s.handleRevokeInvite))
	mux.HandleFunc("DELETE /api/admin/users/{id}", s.withAdmin(s.handleKickUser))

	files := http.FileServerFS(s.static)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// SPA routes: the client resolves /join, /i/<item id> and /u/<user id>.
		// They carry no data; objects are only ever sent to their team over /ws.
		p := r.URL.Path
		if p == "/join" || strings.HasPrefix(p, "/i/") || strings.HasPrefix(p, "/u/") {
			r.URL.Path = "/"
		}
		if p == "/sw.js" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	return mux
}

// --- auth helpers ---

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if t, ok := strings.CutPrefix(h, "Bearer "); ok {
		return t
	}
	return ""
}

func (s *Server) withUser(h func(http.ResponseWriter, *http.Request, store.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.store.UserByToken(bearer(r))
		if errors.Is(err, store.ErrUnauthorized) {
			httpError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			s.internal(w, err)
			return
		}
		h(w, r, u)
	}
}

func (s *Server) withAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t := bearer(r)
		if s.adminToken == "" || subtle.ConstantTimeCompare([]byte(t), []byte(s.adminToken)) != 1 {
			httpError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		h(w, r)
	}
}

// --- public / member handlers ---

// handleInviteInfo tells the join screen which team a link leads to.
func (s *Server) handleInviteInfo(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.InviteTeam(r.PathValue("token"))
	if errors.Is(err, store.ErrInvalidInvite) || errors.Is(err, store.ErrNotFound) {
		httpError(w, http.StatusForbidden, store.ErrInvalidInvite.Error())
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"team": map[string]string{"id": t.ID, "name": t.Name}})
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Invite   string `json:"invite"`
		Callsign string `json:"callsign"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "bad json")
		return
	}
	cs, err := cleanCallsign(req.Callsign)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	u, t, token, err := s.store.Join(req.Invite, cs)
	switch {
	case errors.Is(err, store.ErrInvalidInvite):
		httpError(w, http.StatusForbidden, err.Error())
		return
	case errors.Is(err, store.ErrCallsignTaken):
		httpError(w, http.StatusConflict, "callsign already taken in this team, pick another")
		return
	case err != nil:
		s.internal(w, err)
		return
	}
	s.log.Info("user joined", "team", t.Name, "id", u.ID, "callsign", u.Callsign)
	// Tell the team about the newcomer and their personal folder.
	pub := u.Public()
	s.hub.broadcast(u.TeamID, msg{T: "user", User: &pub})
	if f, err := s.store.Item(u.TeamID, u.FolderID); err == nil {
		s.hub.broadcast(u.TeamID, msg{T: "item", Item: &f})
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u.Public(), "team": t, "token": token})
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, u store.User) {
	t, err := s.store.Team(u.TeamID)
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u.Public(), "team": t})
}

// --- admin handlers ---

type inviteRow struct {
	store.Invite
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

func (s *Server) inviteRow(i store.Invite) inviteRow {
	return inviteRow{Invite: i, URL: s.InviteURL(i.Token), Active: i.Active(time.Now())}
}

func (s *Server) handleListTeams(w http.ResponseWriter, _ *http.Request) {
	teams, err := s.store.Teams()
	if err != nil {
		s.internal(w, err)
		return
	}
	type row struct {
		store.Team
		Members int         `json:"members"`
		Invites []inviteRow `json:"invites"`
	}
	out := []row{}
	for _, t := range teams {
		users, err := s.store.Users(t.ID)
		if err != nil {
			s.internal(w, err)
			return
		}
		invs, err := s.store.Invites(t.ID)
		if err != nil {
			s.internal(w, err)
			return
		}
		r := row{Team: t, Members: len(users), Invites: []inviteRow{}}
		for _, i := range invs {
			r.Invites = append(r.Invites, s.inviteRow(i))
		}
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCreateTeam creates a team; the response carries its invite link.
func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "bad json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 80 {
		httpError(w, http.StatusBadRequest, "name must be 1-80 characters")
		return
	}
	t, inv, err := s.store.CreateTeam(name)
	if err != nil {
		s.internal(w, err)
		return
	}
	s.log.Info("team created", "team", t.Name, "invite", s.InviteURL(inv.Token))
	writeJSON(w, http.StatusOK, map[string]any{"team": t, "invite": s.inviteRow(inv), "url": s.InviteURL(inv.Token)})
}

// handleCreateInvite adds another link to a team, e.g. to rotate a leaked one.
func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TTLHours float64 `json:"ttlHours"` // 0 = never expires
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, "bad json")
			return
		}
	}
	inv, err := s.store.CreateInvite(r.PathValue("id"), time.Duration(req.TTLHours*float64(time.Hour)))
	if errors.Is(err, store.ErrNotFound) {
		httpError(w, http.StatusNotFound, "team not found")
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"invite": s.inviteRow(inv), "url": s.InviteURL(inv.Token)})
}

func (s *Server) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	ok, err := s.store.RevokeInvite(r.PathValue("token"))
	if err != nil {
		s.internal(w, err)
		return
	}
	if !ok {
		httpError(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users(r.PathValue("id"))
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// handleKickUser removes a user: their token stops working, their open
// connections are closed and the rest of their team drops their marker.
func (s *Server) handleKickUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	teamID, err := s.store.RemoveUser(id)
	if errors.Is(err, store.ErrNotFound) {
		httpError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	s.hub.kick(id)
	s.hub.broadcast(teamID, msg{T: "leave", UserID: id})
	s.log.Info("user kicked", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// --- responses ---

func (s *Server) internal(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "err", err)
	httpError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
