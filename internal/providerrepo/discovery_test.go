package providerrepo

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDiscoverRetainsPreviousMetadataWhenRepositoryIsUnavailable(t *testing.T) {
	manifest := validManifest()
	index := repositoryIndex(manifest)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/repository.json" {
			_, _ = w.Write(mustJSON(t, index))
			return
		}
		http.NotFound(w, request)
	}))
	client := TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)})
	settings := Settings{Repositories: []ConfiguredRepository{{URL: server.URL + "/repository.json", Enabled: true}}}
	first := Discover(context.Background(), settings, NewClient(client), nil)
	if len(first) != 1 || !first[0].Available || first[0].Repository == nil {
		t.Fatalf("first discovery = %+v", first)
	}
	server.Close()
	second := Discover(context.Background(), settings, NewClient(client), first)
	if len(second) != 1 || second[0].Available || second[0].Repository == nil || second[0].Error == "" {
		t.Fatalf("outage discovery = %+v", second)
	}
	if len(second[0].Providers) != 1 || second[0].Providers[0].ID != manifest.Provider.ID {
		t.Fatalf("retained repository = %+v", second[0])
	}
}

func TestDiscoverReportsDisabledAndInvalidRepositoriesWithoutFetching(t *testing.T) {
	settings := Settings{Repositories: []ConfiguredRepository{
		{URL: "https://example.test/disabled.json", Enabled: false},
		{URL: "http://example.test/insecure.json", Enabled: true},
	}}
	results := Discover(context.Background(), settings, NewClient(nil), nil)
	if len(results) != 2 || results[0].Available || results[0].Error != "" {
		t.Fatalf("disabled result = %+v", results[0])
	}
	if results[1].Error == "" || !strings.Contains(results[1].Error, "HTTPS") {
		t.Fatalf("invalid result = %+v", results[1])
	}
}

func TestMetadataDiscoveryDoesNotDownloadArtifacts(t *testing.T) {
	manifest := validManifest()
	var artifactRequests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/artifact" {
			artifactRequests.Add(1)
			_, _ = w.Write([]byte("must not be downloaded during discovery"))
			return
		}
		manifest.Provider.Artifacts["linux-amd64"] = Artifact{URL: "https://" + request.Host + "/artifact", SHA256: strings.Repeat("a", 64)}
		_, _ = w.Write(mustJSON(t, repositoryIndex(manifest)))
	}))
	defer server.Close()
	client := TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)})
	settings := Settings{Repositories: []ConfiguredRepository{{URL: server.URL + "/repository.json", Enabled: true}}}
	results := Discover(context.Background(), settings, NewClient(client), nil)
	if len(results) != 1 || results[0].Repository == nil || results[0].Error != "" {
		t.Fatalf("discovery = %+v", results)
	}
	if artifactRequests.Load() != 0 {
		t.Fatalf("artifact requests during metadata discovery = %d", artifactRequests.Load())
	}
}

func repositoryIndex(manifest Manifest) RepositoryIndex {
	return RepositoryIndex{Version: ManifestVersion, Repository: manifest.Repository, Providers: []Provider{manifest.Provider}}
}
