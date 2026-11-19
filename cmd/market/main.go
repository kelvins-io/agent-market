package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kelvins-io/agent-market/internal/api"
	"github.com/kelvins-io/agent-market/internal/discovery"
	"github.com/kelvins-io/agent-market/internal/hostagent"
	"github.com/kelvins-io/agent-market/internal/registry"
)

func main() {
	addr := flag.String("addr", ":13100", "listen address")
	dbPath := flag.String("db", "data/market.db", "sqlite database path")
	staticDir := flag.String("static", "web/dist", "static UI directory")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		log.Fatal(err)
	}

	store, err := registry.New(*dbPath)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}
	defer store.Close()

	disc := discovery.New(store)
	ctx := context.Background()
	host, err := hostagent.New(ctx, store, disc)
	if err != nil {
		log.Fatalf("host agent: %v", err)
	}
	if host == nil {
		log.Printf("WARN: OPENAI_API_KEY unset — /api/v1/chat disabled; discovery and A2A proxy still work")
	} else {
		log.Printf("MarketHost agent ready")
	}

	static := *staticDir
	if _, err := os.Stat(static); err != nil {
		log.Printf("WARN: static dir %s missing — API only (build web/ first)", static)
		static = ""
	}

	srv := &api.Server{
		Store:     store,
		Discovery: disc,
		Host:      host,
		StaticDir: static,
	}

	log.Printf("Agent Market listening on http://127.0.0.1%s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
