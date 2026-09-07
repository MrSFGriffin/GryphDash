package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web
var webFiles embed.FS

func newHandler(c *collector) http.Handler {
	assets := map[string]struct{ path, contentType string }{
		"/":                         {"web/dashboard.html", "text/html; charset=utf-8"},
		"/assets/dashboard.js":      {"web/dashboard.js", "text/javascript; charset=utf-8"},
		"/assets/dashboard.css":     {"web/dashboard.css", "text/css; charset=utf-8"},
		"/assets/gridstack-all.js":  {"web/vendor/gridstack/gridstack-all.js", "text/javascript; charset=utf-8"},
		"/assets/gridstack.min.css": {"web/vendor/gridstack/gridstack.min.css", "text/css; charset=utf-8"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset, found := assets[r.URL.Path]
		if !found && r.URL.Path != "/api/widgets" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path == "/api/widgets" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method == http.MethodHead {
				return
			}
			if err := json.NewEncoder(w).Encode(buildDashboard(c.snapshot())); err != nil {
				log.Printf("encode widgets: %v", err)
			}
			return
		}
		content, err := webFiles.ReadFile(asset.path)
		if err != nil {
			http.Error(w, "Asset unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", asset.contentType)
		http.ServeContent(w, r, asset.path, time.Time{}, bytes.NewReader(content))
	})
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	interval := time.Minute
	if raw := os.Getenv("GRYPHDASH_REFRESH_INTERVAL"); raw != "" {
		var err error
		interval, err = time.ParseDuration(raw)
		if err != nil || interval < 30*time.Second {
			log.Fatal("GRYPHDASH_REFRESH_INTERVAL must be a duration of at least 30s")
		}
	}
	executable := os.Getenv("GRYPHDASH_CODEX_BIN")
	if executable == "" {
		executable = "codex"
	}
	c := &collector{executable: executable}
	c.openRouterKey = os.Getenv("OPENROUTER_API_KEY")
	c.httpClient = &http.Client{Timeout: 15 * time.Second}
	done := make(chan struct{})
	go func() { defer close(done); c.run(ctx, interval) }()

	addr := os.Getenv("GRYPHDASH_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(c),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("GryphDash listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server: %v", err)
	}
	stop()
	<-done
}
