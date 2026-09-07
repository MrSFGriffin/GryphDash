package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web logo.svg
var webFiles embed.FS

func newHandler(c *collector) http.Handler {
	assets := map[string]struct{ path, contentType string }{
		"/":                         {"web/dashboard.html", "text/html; charset=utf-8"},
		"/assets/dashboard.js":      {"web/dashboard.js", "text/javascript; charset=utf-8"},
		"/assets/dashboard.css":     {"web/dashboard.css", "text/css; charset=utf-8"},
		"/assets/gridstack-all.js":  {"web/vendor/gridstack/gridstack-all.js", "text/javascript; charset=utf-8"},
		"/assets/gridstack.min.css": {"web/vendor/gridstack/gridstack.min.css", "text/css; charset=utf-8"},
		"/assets/logo.svg":          {"logo.svg", "image/svg+xml"},
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
	mode := "web"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if mode == "help" || mode == "-h" || mode == "--help" {
		printUsage()
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch mode {
	case "web":
		err = runWeb(ctx)
	case "tui":
		err = runTUI(ctx)
	default:
		printUsage()
		os.Exit(2)
	}
	if err != nil {
		log.Print(err)
	}
}

func printUsage() {
	fmt.Println("Usage: gryphdash [web|tui]")
	fmt.Println("  web  serve the browser dashboard (default)")
	fmt.Println("  tui  display the dashboard in the terminal")
}

func newCollector() *collector {
	executable := os.Getenv("GRYPHDASH_CODEX_BIN")
	if executable == "" {
		executable = "codex"
	}
	return &collector{executable: executable, openRouterKey: os.Getenv("OPENROUTER_API_KEY"), httpClient: &http.Client{Timeout: 15 * time.Second}}
}

func refreshInterval() (time.Duration, error) {
	interval := time.Minute
	if raw := os.Getenv("GRYPHDASH_REFRESH_INTERVAL"); raw != "" {
		var err error
		interval, err = time.ParseDuration(raw)
		if err != nil || interval < 30*time.Second {
			return 0, errors.New("GRYPHDASH_REFRESH_INTERVAL must be a duration of at least 30s")
		}
	}
	return interval, nil
}

func runWeb(ctx context.Context) error {
	interval, err := refreshInterval()
	if err != nil {
		return err
	}
	c := newCollector()
	done := make(chan struct{})
	go func() { defer close(done); c.run(ctx, interval) }()
	addr := os.Getenv("GRYPHDASH_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{Addr: addr, Handler: newHandler(c), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("GryphDash listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-done
	return nil
}
