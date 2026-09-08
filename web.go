package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"gryphdash/internal/app"
	"gryphdash/internal/config"
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
			if err := json.NewEncoder(w).Encode(buildDashboard(c.Snapshot())); err != nil {
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

func runWeb(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	c := newCollector(cfg)
	runtime, err := app.New(app.Options{
		Collector: c,
		Handler:   newHandler(c),
		Address:   cfg.Address,
		Interval:  cfg.RefreshInterval,
	})
	if err != nil {
		return err
	}
	log.Printf("GryphDash listening on http://%s", cfg.Address)
	return runtime.Run(ctx)
}
