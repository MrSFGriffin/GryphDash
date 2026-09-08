package web

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"gryphdash/internal/collector"
	"gryphdash/internal/dashboard"
)

type SnapshotProvider interface {
	Snapshot() collector.Snapshot
}

func NewHandler(provider SnapshotProvider, assets fs.FS) http.Handler {
	assetPaths := map[string]struct{ path, contentType string }{
		"/":                         {"dashboard.html", "text/html; charset=utf-8"},
		"/assets/dashboard.js":      {"dashboard.js", "text/javascript; charset=utf-8"},
		"/assets/dashboard.css":     {"dashboard.css", "text/css; charset=utf-8"},
		"/assets/gridstack-all.js":  {"vendor/gridstack/gridstack-all.js", "text/javascript; charset=utf-8"},
		"/assets/gridstack.min.css": {"vendor/gridstack/gridstack.min.css", "text/css; charset=utf-8"},
		"/assets/logo.svg":          {"logo.svg", "image/svg+xml"},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset, found := assetPaths[r.URL.Path]
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
			if err := json.NewEncoder(w).Encode(dashboard.BuildDashboard(provider.Snapshot())); err != nil {
				log.Printf("encode widgets: %v", err)
			}
			return
		}
		content, err := fs.ReadFile(assets, asset.path)
		if err != nil {
			http.Error(w, "Asset unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", asset.contentType)
		http.ServeContent(w, r, asset.path, time.Time{}, bytes.NewReader(content))
	})
}
