package server

import (
	_ "embed"
	"errors"
	"net/http"
	"strings"
	"text/template"

	"sitaw/internal/store"
)

// Agent management for people (session auth): create, list, revoke. An agent
// is a sub-user named <OWNER>-<NATO>; creating one returns its token once,
// inside a ready-to-paste prompt.

// agent.md: the agent's instructions (identity, API, situational-awareness and
// GIS guidance), fetched with its token from GET /agent.
//
//go:embed agent.md
var instructionsSource string

var instructionsTmpl = template.Must(template.New("agent").Parse(instructionsSource))

// The prompt is deliberately minimal: who you are, the token, where the rest is.
var promptTmpl = template.Must(template.New("prompt").Parse(`You are {{.Callsign}}, an AI agent for {{.Owner}} on the sitaw team map.
Your access token: {{.Token}}
Fetch your instructions and follow them:
curl -fsSL -H "Authorization: Bearer {{.Token}}" {{.Base}}/agent
`))

// handleAgentInstructions serves the caller's personalised instructions.
func (s *Server) handleAgentInstructions(w http.ResponseWriter, r *http.Request, u store.User) {
	t, err := s.store.Team(u.TeamID)
	if err != nil {
		s.internal(w, err)
		return
	}
	owner := u.Callsign
	if u.IsAgent() {
		if users, err := s.store.Users(u.TeamID); err == nil {
			for _, o := range users {
				if o.ID == u.OwnerID {
					owner = o.Callsign
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	_ = instructionsTmpl.Execute(w, map[string]string{
		"Base": s.publicURL(r), "Callsign": u.Callsign, "Owner": owner, "Team": t.Name, "FolderID": u.FolderID,
	})
}

// humanOnly keeps agents from managing agents and teams.
func (s *Server) humanOnly(h func(http.ResponseWriter, *http.Request, store.User)) http.HandlerFunc {
	return s.withUser(func(w http.ResponseWriter, r *http.Request, u store.User) {
		if u.IsAgent() {
			httpError(w, http.StatusForbidden, "agents cannot manage agents or teams")
			return
		}
		h(w, r, u)
	})
}

func (s *Server) handleListAgents(w http.ResponseWriter, _ *http.Request, u store.User) {
	list, err := s.store.Agents(u.ID)
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request, u store.User) {
	a, token, err := s.store.CreateAgent(u)
	if err != nil {
		s.internal(w, err)
		return
	}
	t, err := s.store.Team(u.TeamID)
	if err != nil {
		s.internal(w, err)
		return
	}
	var prompt strings.Builder
	_ = promptTmpl.Execute(&prompt, map[string]string{
		"Callsign": a.Callsign, "Owner": u.Callsign, "Base": s.publicURL(r), "Token": token,
	})
	s.log.Info("agent created", "agent", a.Callsign, "owner", u.Callsign, "team", t.Name)
	// The team sees the new member and its folder right away.
	pub := a.Public()
	s.hub.broadcast(u.TeamID, msg{T: "user", User: &pub})
	if f, err := s.store.Item(u.TeamID, a.FolderID); err == nil {
		s.hub.broadcast(u.TeamID, msg{T: "item", Item: &f})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent": store.Agent{ID: a.ID, Callsign: a.Callsign, FolderID: a.FolderID, CreatedAt: a.CreatedAt},
		"token": token, "prompt": prompt.String(),
	})
}

func (s *Server) handleRevokeAgent(w http.ResponseWriter, r *http.Request, u store.User) {
	pub, gone, err := s.store.RevokeAgent(u.ID, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpError(w, http.StatusNotFound, "no such agent of yours")
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	s.hub.kick(pub.ID)
	s.hub.broadcast(u.TeamID, msg{T: "user", User: &pub})
	for i := range gone {
		s.hub.broadcast(u.TeamID, msg{T: "item", Item: &gone[i]})
	}
	s.log.Info("agent revoked", "agent", pub.Callsign, "owner", u.Callsign)
	w.WriteHeader(http.StatusNoContent)
}
