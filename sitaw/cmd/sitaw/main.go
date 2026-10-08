// Command sitaw runs the sitaw server: a lightweight, multi-team ATAK-like
// situational-awareness backend with an embedded web client.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sitaw/internal/server"
	"sitaw/internal/store"
	"sitaw/web"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "data/sitaw.db", "SQLite database file")
	legacy := flag.String("import-json", "data/sitaw.json", "pre-SQLite state file, imported once as team \"default\" if the database has no teams")
	baseURL := flag.String("base-url", "http://localhost:8080", "public URL, used to print invite links")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log, *addr, *dbPath, *legacy, *baseURL); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, addr, dbPath, legacy, baseURL string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
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
				log.Info("invite", "team", t.Name, "url", srv.InviteURL(inv.Token))
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hs := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr, "db", dbPath)
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
