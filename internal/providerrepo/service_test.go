package providerrepo

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestServiceRepositoryManagementAndStatuses(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "repositories.json")
	settings := Settings{Repositories: []ConfiguredRepository{{URL: "https://core.example/manifest.json", Enabled: true}}}
	if err := SaveSettings(settingsPath, settings); err != nil {
		t.Fatal(err)
	}
	manifest := installManifest("https://core.example/provider", []byte("provider"), "1.0.0")
	service := NewService(settingsPath, t.TempDir(), []Discovery{discoveryForManifest(settings.Repositories[0].URL, manifest)})
	statuses := service.Statuses()
	if len(statuses) != 1 || statuses[0].State != StateAvailable {
		t.Fatalf("statuses = %+v", statuses)
	}
	if _, err := service.AddRepository("https://extra.example/manifest.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetRepositoryEnabled("https://extra.example/manifest.json", false); err != nil {
		t.Fatal(err)
	}
	repositories := service.Repositories()
	if len(repositories) != 2 || repositories[1].Enabled {
		t.Fatalf("repositories = %+v", repositories)
	}
	if _, err := service.RemoveRepository("https://extra.example/manifest.json"); err != nil {
		t.Fatal(err)
	}
	if len(service.Repositories()) != 1 {
		t.Fatalf("repositories after remove = %+v", service.Repositories())
	}
}

func TestServiceReportsStaleMetadata(t *testing.T) {
	manifest := installManifest("https://stale.example/provider", []byte("provider"), "1.0.0")
	discovery := discoveryForManifest("https://stale.example/repository.json", manifest)
	discovery.Error = "repository unavailable"
	service := NewService(filepath.Join(t.TempDir(), "repositories.json"), t.TempDir(), []Discovery{discovery})
	statuses := service.Statuses()
	if len(statuses) != 1 || statuses[0].State != StateStale || !strings.Contains(statuses[0].Error, "unavailable") {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestServiceInstallUsesManagedCacheAndReportsInstalling(t *testing.T) {
	payload := []byte("#!/bin/sh\nexit 0\n")
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release; _, _ = w.Write(payload) }))
	defer server.Close()
	manifest := installManifest(server.URL+"/provider", payload, "1.0.0")
	settingsPath := filepath.Join(t.TempDir(), "repositories.json")
	if err := SaveSettings(settingsPath, Settings{Repositories: []ConfiguredRepository{{URL: "https://example.test/repository.json", Enabled: true}}}); err != nil {
		t.Fatal(err)
	}
	service := NewService(settingsPath, t.TempDir(), []Discovery{discoveryForManifest("https://example.test/repository.json", manifest)})
	service.manager.HTTPClient = TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)})
	if err := service.Install(context.Background(), "https://example.test/repository.json", manifest.Provider.ID); err != nil {
		t.Fatal(err)
	}
	if statuses := service.Statuses(); len(statuses) != 1 || statuses[0].State != StateInstalling {
		t.Fatalf("statuses = %+v", statuses)
	}
	close(release)
	path, err := service.manager.InstalledPath("https://example.test/repository.json", manifest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("managed provider was not installed")
		}
		time.Sleep(time.Millisecond)
	}
}

func discoveryForManifest(url string, manifest Manifest) Discovery {
	repository := manifest.Repository
	return Discovery{URL: url, Enabled: true, Available: true, Repository: &repository, Providers: []Provider{manifest.Provider}}
}
