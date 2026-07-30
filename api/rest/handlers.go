package rest

import (
	"encoding/json"
	"net/http"

	"autoapp/internal/actionlog"
	"autoapp/internal/agent"
	"autoapp/internal/db"
	"autoapp/internal/uistate"
	"autoapp/internal/ws"
)

// Ensure actionlog is used (for EventType in actionRequest).
var _ = actionlog.EventRowClick

type queryRequest struct {
	SQL string `json:"sql"`
}

type Handlers struct {
	exec      *db.Executor
	inspector *db.Inspector
	log       *actionlog.Log
	ui        *uistate.Manager
	hub       *ws.Hub
	debouncer *agent.Debouncer // nil when agent is disabled
}

func NewHandlers(exec *db.Executor, inspector *db.Inspector, log *actionlog.Log, ui *uistate.Manager, hub *ws.Hub, debouncer *agent.Debouncer) *Handlers {
	return &Handlers{exec: exec, inspector: inspector, log: log, ui: ui, hub: hub, debouncer: debouncer}
}

func (h *Handlers) Query(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SQL == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result := h.exec.Query(r.Context(), req.SQL)
	writeJSON(w, result)
}

func (h *Handlers) Schema(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	if table == "" {
		tables, err := h.inspector.Tables(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tables)
		return
	}
	writeJSON(w, h.inspector.Inspect(r.Context(), table))
}

func (h *Handlers) UITree(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.ui.Tree())
}

type actionRequest struct {
	Type    actionlog.EventType    `json:"type"`
	Payload map[string]any         `json:"payload"`
}

func (h *Handlers) Action(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Type == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	ev := h.log.Append(req.Type, req.Payload)
	if h.debouncer != nil {
		h.debouncer.Trigger(ev)
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
