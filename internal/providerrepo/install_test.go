package providerrepo

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestInstallVerifiesAndAtomicallyInstallsProvider(t *testing.T) {
	payload := []byte("#!/bin/sh\necho managed-provider\n")
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/provider" {
			http.NotFound(w, request)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	manifest := installManifest(server.URL+"/provider", payload, "1.2.3")
	manager := NewManager(t.TempDir(), TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)}))
	installed, err := manager.Install(context.Background(), "https://example.test/repository.json", manifest, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 || installed.Path == "" {
		t.Fatalf("requests = %d, installed = %+v", requests.Load(), installed)
	}
	data, err := os.ReadFile(installed.Path)
	if err != nil || string(data) != string(payload) {
		t.Fatalf("installed data = %q, error = %v", data, err)
	}
	info, err := os.Stat(installed.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("permissions = %o, want 700", info.Mode().Perm())
	}
	if strings.Contains(installed.Path, "1.2.3/linux-amd64") == false {
		t.Fatalf("cache path does not include version/platform: %s", installed.Path)
	}
	if ok, err := manager.IsInstalled("https://example.test/repository.json", manifest, "linux", "amd64"); err != nil || !ok {
		t.Fatalf("IsInstalled() = %v, %v", ok, err)
	}
}

func TestInstallFailurePreservesExistingBinaryAndCleansTemporaryFiles(t *testing.T) {
	oldPayload := []byte("old provider")
	newPayload := []byte("new provider")
	var payload atomic.Value
	payload.Store(oldPayload)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/status" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write(payload.Load().([]byte))
	}))
	defer server.Close()
	manager := NewManager(t.TempDir(), TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)}))
	repositoryURL := "https://example.test/repository.json"
	good := installManifest(server.URL+"/provider", oldPayload, "1.0.0")
	installed, err := manager.Install(context.Background(), repositoryURL, good, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(installed.Path), ".install-crash"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	badChecksum := installManifest(server.URL+"/provider", newPayload, "1.0.0")
	if _, err := manager.Install(context.Background(), repositoryURL, badChecksum, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum error = %v", err)
	}
	data, err := os.ReadFile(installed.Path)
	if err != nil || string(data) != string(oldPayload) {
		t.Fatalf("existing data = %q, error = %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(installed.Path), ".install-crash")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary recovery file error = %v", err)
	}
	statusManifest := installManifest(server.URL+"/status", newPayload, "2.0.0")
	if _, err := manager.Install(context.Background(), repositoryURL, statusManifest, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("HTTP error = %v", err)
	}
}

func TestInstallRejectsTruncatedAndOversizedDownloads(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()
	manager := NewManager(t.TempDir(), TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)}))
	manifest := installManifest(server.URL+"/provider", []byte("the expected payload"), "1.0.0")
	if _, err := manager.Install(context.Background(), "https://example.test/repository.json", manifest, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("truncated error = %v", err)
	}
	manager.MaxArtifactBytes = 4
	manifest = installManifest(server.URL+"/provider", []byte("short"), "1.0.1")
	if _, err := manager.Install(context.Background(), "https://example.test/repository.json", manifest, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversized error = %v", err)
	}
}

func TestUpdateInstallsNewVersionBeforeRemovingOldAndRemoveIsIdempotent(t *testing.T) {
	payload := []byte("provider")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	manager := NewManager(t.TempDir(), TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)}))
	repositoryURL := "https://example.test/repository.json"
	v1 := installManifest(server.URL+"/provider", payload, "1.0.0")
	old, err := manager.Install(context.Background(), repositoryURL, v1, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	v2 := installManifest(server.URL+"/provider", payload, "2.0.0")
	updated, err := manager.Update(context.Background(), repositoryURL, v2, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old version still exists, stat error = %v", err)
	}
	if _, err := os.Stat(updated.Path); err != nil {
		t.Fatalf("new version missing: %v", err)
	}
	if err := manager.Remove(repositoryURL, v2.Repository.ID, v2.Provider.ID, v2.Provider.Version, "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Remove(repositoryURL, v2.Repository.ID, v2.Provider.ID, v2.Provider.Version, "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
}

func installManifest(artifactURL string, payload []byte, version string) Manifest {
	hash := sha256.Sum256(payload)
	manifest := validManifest()
	manifest.Repository.ID = "core"
	manifest.Provider.ID = "currency"
	manifest.Provider.Version = version
	manifest.Provider.Artifacts["linux-amd64"] = Artifact{URL: artifactURL, SHA256: hex.EncodeToString(hash[:])}
	return manifest
}
