package providerrepo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientFetchesMetadataWithoutRequestingArtifacts(t *testing.T) {
	manifest := validManifest()
	manifest.Provider.Artifacts["linux-amd64"] = Artifact{URL: "https://artifact.example/provider", SHA256: strings.Repeat("a", 64)}
	data := mustJSON(t, repositoryIndex(manifest))
	artifactRequested := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repository.json" {
			artifactRequested = true
			http.NotFound(w, request)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	client := TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)})
	got, err := NewClient(client).FetchRepository(context.Background(), server.URL+"/repository.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 1 || got.Providers[0].ID != manifest.Provider.ID || artifactRequested {
		t.Fatalf("repository = %+v, artifact requested = %v", got, artifactRequested)
	}
}

func TestClientRejectsHTTPAndOversizedManifests(t *testing.T) {
	if _, err := NewClient(nil).FetchRepository(context.Background(), "http://example.test/repository.json"); err == nil {
		t.Fatal("HTTP repository unexpectedly accepted")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 32)))
	}))
	defer server.Close()
	client := TLSClient(&tls.Config{RootCAs: serverCertPool(t, server)})
	_, err := (Client{HTTPClient: client, MaxManifestSize: 8}).FetchRepository(context.Background(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversized manifest error = %v", err)
	}
}

func TestClientRejectsHTTPSRedirectToHTTP(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer httpServer.Close()
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, httpServer.URL, http.StatusFound)
	}))
	defer tlsServer.Close()
	client := TLSClient(&tls.Config{RootCAs: serverCertPool(t, tlsServer)})
	_, err := NewClient(client).FetchRepository(context.Background(), tlsServer.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("redirect error = %v", err)
	}
}

func serverCertPool(t *testing.T, server *httptest.Server) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]})
	if !pool.AppendCertsFromPEM(certificate) {
		t.Fatal("failed to load test certificate")
	}
	return pool
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
