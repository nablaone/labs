// Command sitaw runs the sitaw server: a lightweight, multi-team ATAK-like
// situational-awareness backend with an embedded web client.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"sitaw/internal/server"
	"sitaw/internal/store"
	"sitaw/web"
)

// Settings come from flags, which default to environment variables:
//
//	SITAW_ADDR         listen address (:8080)
//	SITAW_DB           SQLite file (data/sitaw.db)
//	SITAW_BASE_URL     external address clients use, e.g. https://<machine>.<tailnet>.ts.net.
//	                   Used in invite links, agent prompts and agent instructions.
//	                   Unset: derived from each request (fine without a proxy/tunnel).
//	SITAW_IMPORT_JSON  pre-SQLite state file to import once (data/sitaw.json)
//	SITAW_ADMIN_TOKEN  bearer token for /api/admin (random per run if unset)
func main() {
	addr := flag.String("addr", env("SITAW_ADDR", ":8080"), "listen address [SITAW_ADDR]")
	dbPath := flag.String("db", env("SITAW_DB", "data/sitaw.db"), "SQLite database file [SITAW_DB]")
	legacy := flag.String("import-json", env("SITAW_IMPORT_JSON", "data/sitaw.json"),
		"pre-SQLite state file, imported once as team \"default\" if the database has no teams [SITAW_IMPORT_JSON]")
	baseURL := flag.String("base-url", env("SITAW_BASE_URL", ""),
		"external address clients use, e.g. https://host.tailnet.ts.net; empty = derive from each request [SITAW_BASE_URL]")
	healthcheck := flag.Bool("healthcheck", false, "check that a server on -addr is healthy (exit 0/1) and quit; for container healthchecks")
	flag.Parse()
	if *healthcheck {
		os.Exit(checkHealth(*addr))
	}
	*baseURL = strings.TrimRight(*baseURL, "/")
	if *baseURL != "" && !strings.HasPrefix(*baseURL, "http://") && !strings.HasPrefix(*baseURL, "https://") {
		fmt.Fprintf(os.Stderr, "base URL must start with http:// or https://, got %q\n", *baseURL)
		os.Exit(2)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log, *addr, *dbPath, *legacy, *baseURL); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func run(log *slog.Logger, addr, dbPath, legacy, baseURL string) error {
	if err := checkDataDir(filepath.Dir(dbPath)); err != nil {
		return err
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	adminToken := os.Getenv("SITAW_ADMIN_TOKEN")
	if adminToken == "" {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		adminToken = hex.EncodeToString(b)
		log.Warn("SITAW_ADMIN_TOKEN not set, using a random one for this run", "adminToken", adminToken)
	}
	srv := server.New(st, web.Static(), adminToken, baseURL, log)

	// Links printed at startup: the external address, or this machine as a hint.
	printBase := baseURL
	if printBase == "" {
		_, port, _ := net.SplitHostPort(addr)
		printBase = "http://localhost:" + port
		log.Info("SITAW_BASE_URL not set: links use the address each client connects to; " +
			"set it when serving through a proxy or tunnel (e.g. tailscale serve)")
	}

	if err := bootstrap(log, st, legacy); err != nil {
		return err
	}
	// Print every team's active links so there is always a way in.
	teams, err := st.Teams()
	if err != nil {
		return err
	}
	for _, t := range teams {
		invs, err := st.Invites(t.ID)
		if err != nil {
			return err
		}
		for _, inv := range invs {
			if inv.Active(time.Now()) {
				log.Info("invite", "team", t.Name, "url", printBase+"/join?t="+inv.Token)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hs := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr, "db", dbPath, "external", printBase)
		errc <- hs.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hs.Shutdown(shutdownCtx)
}

// bootstrap makes sure a fresh database has a team: it imports the old JSON
// state if present, otherwise creates an empty team called "default".
func bootstrap(log *slog.Logger, st *store.Store, legacy string) error {
	teams, err := st.Teams()
	if err != nil || len(teams) > 0 {
		return err
	}
	if _, err := os.Stat(legacy); err == nil {
		t, err := st.ImportLegacyJSON(legacy, "default")
		if err != nil {
			return err
		}
		log.Info("imported legacy JSON state", "file", legacy, "team", t.Name)
		return nil
	}
	t, _, err := st.CreateTeam("default")
	if err == nil {
		log.Info("created team", "team", t.Name)
	}
	return err
}

// checkHealth calls /healthz on the local server (the image has no curl).
func checkHealth(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad addr:", err)
		return 1
	}
	c := http.Client{Timeout: 2 * time.Second}
	res, err := c.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "status", res.StatusCode)
		return 1
	}
	return 0
}

// checkDataDir makes sure the database directory exists and is writable, with
// an error that says what to do (a bind mount created by Docker is root's).
func checkDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("data directory %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return fmt.Errorf("data directory %s is not writable by uid %d (%w); "+
			"chown it to that user, or run sitaw as its owner (compose: SITAW_UID/SITAW_GID)", dir, os.Getuid(), err)
	}
	f.Close()
	return os.Remove(f.Name())
}
