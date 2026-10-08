package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sitaw/internal/store"
)

// Team endpoints for people (session auth), behind the INFO panel: the
// team's invite link, and creating a new team.

// handleTeamInvite returns the team's newest active invite. A team whose
// links were all revoked gets none: only an admin issues a new one.
func (s *Server) handleTeamInvite(w http.ResponseWriter, _ *http.Request, u store.User) {
	invs, err := s.store.Invites(u.TeamID)
	if err != nil {
		s.internal(w, err)
		return
	}
	now := time.Now()
	for i := len(invs) - 1; i >= 0; i-- {
		if invs[i].Active(now) {
			writeJSON(w, http.StatusOK, map[string]any{"token": invs[i].Token, "expiresAt": invs[i].ExpiresAt})
			return
		}
	}
	httpError(w, http.StatusNotFound, "this team has no active invite link, ask an admin for one")
}

// handleUserCreateTeam lets any person start a new team. The caller stays in
// their team; they join the new one through its invite link, like anyone else.
func (s *Server) handleUserCreateTeam(w http.ResponseWriter, r *http.Request, u store.User) {
	if !s.limiter.allow(u.ID, true) {
		w.Header().Set("Retry-After", "60")
		httpError(w, http.StatusTooManyRequests, "too many requests, try again in a minute")
		return
	}
	name, ok := decodeTeamName(w, r)
	if !ok {
		return
	}
	t, inv, err := s.store.CreateTeam(name)
	if err != nil {
		s.internal(w, err)
		return
	}
	s.log.Info("team created", "team", t.Name, "by", u.Callsign, "from", u.TeamID)
	writeJSON(w, http.StatusOK, map[string]any{"team": t, "token": inv.Token})
}

// decodeTeamName reads {"name": ...} and writes the error response itself.
func decodeTeamName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "bad json")
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 80 {
		httpError(w, http.StatusBadRequest, "name must be 1-80 characters")
		return "", false
	}
	return name, true
}
