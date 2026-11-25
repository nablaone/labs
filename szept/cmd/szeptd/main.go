// szeptd is the szept mailbox server.
//
//	szeptd serve  -data ./data -addr :8080 -url https://szept.home.lan
//	szeptd invite -data ./data -url https://szept.home.lan
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"szept/proto"
	"szept/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "serve":
		serve(os.Args[2:])
	case "invite":
		invite(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: szeptd serve|invite [flags]  (-h for flags)")
	os.Exit(2)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	data := fs.String("data", "./data", "data directory")
	addr := fs.String("addr", ":8080", "listen address")
	url := fs.String("url", "", "public base URL (used in blob URLs)")
	pushAllow := fs.String("push-allow", "", "comma-separated allowed push URL prefixes (empty: any https URL)")
	cert := fs.String("tls-cert", "", "TLS certificate (omit when behind a reverse proxy)")
	key := fs.String("tls-key", "", "TLS key")
	trustProxy := fs.Bool("trust-proxy", false, "trust X-Forwarded-For (only behind your own proxy)")
	fs.Parse(args)

	cfg := server.Config{DataDir: *data, PublicURL: *url, TrustProxy: *trustProxy}
	if *pushAllow != "" {
		cfg.PushAllow = strings.Split(*pushAllow, ",")
	}
	s, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go s.Janitor(ctx, time.Minute)

	hs := &http.Server{
		Addr:              *addr,
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          log.New(os.Stderr, "http: ", 0),
	}
	go func() {
		<-ctx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sc)
	}()
	log.Printf("szeptd listening on %s, data in %s", *addr, *data)
	if *cert != "" {
		err = hs.ListenAndServeTLS(*cert, *key)
	} else {
		err = hs.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func invite(args []string) {
	fs := flag.NewFlagSet("invite", flag.ExitOnError)
	data := fs.String("data", "./data", "data directory")
	url := fs.String("url", "", "public base URL of this server (required)")
	fs.Parse(args)
	if *url == "" {
		log.Fatal("-url is required")
	}
	if err := os.MkdirAll(*data, 0o700); err != nil {
		log.Fatal(err)
	}
	st, err := server.OpenStore(filepath.Join(*data, "szept.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	inv, exp, err := st.CreateInvite(context.Background(), 0, 0, proto.InviteTTL)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("szept-invite:%s#%s\n", strings.TrimRight(*url, "/"), base64.RawURLEncoding.EncodeToString(inv))
	fmt.Fprintf(os.Stderr, "single use, expires %s\n", exp.Format(time.DateTime))
}
