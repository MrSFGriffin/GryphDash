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
	"gryphdash/internal/providerrepo"
)

type SnapshotProvider interface {
	Snapshot() collector.Snapshot
}

func NewHandler(provider SnapshotProvider, assets fs.FS) http.Handler {
	return NewHandlerWithCatalog(provider, assets, dashboard.Catalog())
}

func NewHandlerWithCatalog(provider SnapshotProvider, assets fs.FS, catalog dashboard.WidgetCatalog) http.Handler {
	return NewHandlerWithCatalogAndRepositories(provider, assets, catalog, nil)
}

func NewHandlerWithCatalogAndRepositories(provider SnapshotProvider, assets fs.FS, catalog dashboard.WidgetCatalog, repositories []providerrepo.Discovery) http.Handler {
	return NewHandlerWithCatalogAndRepositoriesAndService(provider, assets, catalog, repositories, nil)
}

func NewHandlerWithCatalogAndRepositoriesAndService(provider SnapshotProvider, assets fs.FS, catalog dashboard.WidgetCatalog, repositories []providerrepo.Discovery, service *providerrepo.Service) http.Handler {
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
		if !found && r.URL.Path != "/api/widgets" && r.URL.Path != "/api/provider-repositories" && r.URL.Path != "/api/provider-status" {
			http.NotFound(w, r)
			return
		}
		mutating := r.URL.Path == "/api/provider-repositories" || r.URL.Path == "/api/provider-status"
		if !mutating && r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path == "/api/provider-repositories" {
			if service != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
				handleRepositoryAction(w, r, service)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method == http.MethodHead {
				return
			}
			if err := json.NewEncoder(w).Encode(repositories); err != nil {
				log.Printf("encode provider repositories: %v", err)
			}
			return
		}
		if r.URL.Path == "/api/provider-status" {
			if service == nil {
				http.Error(w, "Provider management unavailable", http.StatusNotImplemented)
				return
			}
			if r.Method == http.MethodPost || r.Method == http.MethodDelete {
				handleProviderAction(w, r, service)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method == http.MethodHead {
				return
			}
			_ = json.NewEncoder(w).Encode(service.Statuses())
			return
		}
		if r.URL.Path == "/api/widgets" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method == http.MethodHead {
				return
			}
			if err := json.NewEncoder(w).Encode(dashboard.BuildDashboardWithCatalog(provider.Snapshot(), catalog)); err != nil {
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

type repositoryAction struct {
	URL     string `json:"url"`
	Enabled *bool  `json:"enabled"`
}
type providerAction struct {
	Operation     string `json:"operation"`
	RepositoryURL string `json:"repositoryUrl"`
	ProviderID    string `json:"providerId"`
	Version       string `json:"version,omitempty"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(value); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func handleRepositoryAction(w http.ResponseWriter, r *http.Request, service *providerrepo.Service) {
	if r.Method == http.MethodPost {
		var request repositoryAction
		if !decodeJSON(w, r, &request) {
			return
		}
		if request.URL == "" {
			http.Error(w, "Repository URL is required", http.StatusBadRequest)
			return
		}
		repositories, err := service.AddRepository(request.URL)
		writeRepositoryResult(w, repositories, err)
		return
	}
	if r.Method == http.MethodPatch {
		var request repositoryAction
		if !decodeJSON(w, r, &request) || request.Enabled == nil {
			http.Error(w, "URL and enabled are required", http.StatusBadRequest)
			return
		}
		repositories, err := service.SetRepositoryEnabled(request.URL, *request.Enabled)
		writeRepositoryResult(w, repositories, err)
		return
	}
	if r.Method == http.MethodDelete {
		var request repositoryAction
		if !decodeJSON(w, r, &request) {
			return
		}
		repositories, err := service.RemoveRepository(request.URL)
		writeRepositoryResult(w, repositories, err)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func writeRepositoryResult(w http.ResponseWriter, repositories []providerrepo.Discovery, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(repositories)
}

func handleProviderAction(w http.ResponseWriter, r *http.Request, service *providerrepo.Service) {
	var request providerAction
	if !decodeJSON(w, r, &request) {
		return
	}
	var err error
	switch {
	case r.Method == http.MethodDelete || request.Operation == "remove":
		err = service.RemoveProvider(request.RepositoryURL, request.ProviderID, request.Version)
	case request.Operation == "update":
		err = service.Update(r.Context(), request.RepositoryURL, request.ProviderID)
	case request.Operation == "install" || request.Operation == "":
		err = service.Install(r.Context(), request.RepositoryURL, request.ProviderID)
	default:
		http.Error(w, "Unknown provider operation", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(service.Statuses())
}
