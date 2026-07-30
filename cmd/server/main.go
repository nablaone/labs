package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"time"

	"autoapp/api/rest"
	"autoapp/internal/actionlog"
	"autoapp/internal/agent"
	"autoapp/internal/db"
	"autoapp/internal/discovery"
	"autoapp/internal/schemacache"
	"autoapp/internal/uistate"
	"autoapp/internal/ws"
)

func main() {
	loadDotEnv(".env")

	ctx := context.Background()

	pool, err := db.NewPool(ctx)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	log.Println("database connected")

	exec := db.NewExecutor(pool)
	inspector := db.NewInspector(pool)
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	alogFile := os.Getenv("ACTION_LOG_FILE")
	if alogFile == "" {
		alogFile = dataDir + "/action-log.json"
	}
	alog := actionlog.LoadOrNew(alogFile, 200)

	uiFile := os.Getenv("UI_STATE_FILE")
	if uiFile == "" {
		uiFile = dataDir + "/ui-state.json"
	}
	ui := uistate.LoadOrNew(uiFile)
	hub := ws.NewHub()

	// Seed the default editor page if not already in the loaded state
	ui.Add(uistate.ComponentSpec{
		ID:    "page-editor",
		Type:  "page",
		Label: "Query Editor",
	}) // error ignored — means it was already loaded from file

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Println("WARNING: ANTHROPIC_API_KEY not set — agent disabled")
	}

	schemaFile := dataDir + "/schema.json"
	sc, err := schemacache.LoadOrRefresh(ctx, inspector, apiKey, schemaFile)
	if err != nil {
		log.Printf("WARNING: schema cache failed: %v", err)
	}

	discFile := dataDir + "/discovery.json"
	disc := discovery.LoadOrNew(discFile)

	var debouncer *agent.Debouncer
	if apiKey != "" {
		a := agent.New(apiKey, exec, inspector, sc, alog, ui, hub)
		debouncer = agent.NewDebouncer(a, 300)

		pollInterval := 1 * time.Second
		poller := agent.NewPoller(debouncer, exec, disc, pollInterval)
		poller.Start(ctx)

		log.Printf("agent enabled (polling pg_stat_statements every %s)", pollInterval)
	}

	h := rest.NewHandlers(exec, inspector, alog, ui, hub, debouncer)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /query", h.Query)
	mux.HandleFunc("POST /action", h.Action)
	mux.HandleFunc("GET /schema", h.Schema)
	mux.HandleFunc("GET /schema/{table}", h.Schema)
	mux.HandleFunc("GET /ui", h.UITree)
	mux.HandleFunc("GET /ws", hub.ServeWS)
	mux.Handle("GET /", http.FileServer(http.Dir("./frontend/dist")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

// loadDotEnv reads KEY=VALUE pairs from a .env file into the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
