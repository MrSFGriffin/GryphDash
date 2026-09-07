package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRead(t *testing.T) {
	var gotAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/key":
			_, _ = w.Write([]byte(`{"data":{"usage_monthly":12.5,"limit_remaining":37.5,"expires_at":"2027-12-31T23:59:59Z"}}`))
		case "/credits":
			_, _ = w.Write([]byte(`{"data":{"total_credits":100,"total_usage":25}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	data := (Client{Key: "test-key", HTTPClient: server.Client(), BaseURL: server.URL}).Read(context.Background())
	if data.Key.Error != "" || data.Credits.Error != "" {
		t.Fatalf("unexpected errors: %+v %+v", data.Key, data.Credits)
	}
	if data.Key.Data["usage_monthly"] != float64(12.5) || data.Credits.Data["total_credits"] != float64(100) {
		t.Fatalf("unexpected response: %+v %+v", data.Key.Data, data.Credits.Data)
	}
	if len(gotAuth) != 2 || gotAuth[0] != "Bearer test-key" || gotAuth[1] != "Bearer test-key" {
		t.Fatalf("authorization headers: %v", gotAuth)
	}
}

func TestClientErrors(t *testing.T) {
	data := (Client{}).Read(context.Background())
	if data.Key.Error != "OPENROUTER_API_KEY is not configured" || data.Credits.Error == "" {
		t.Fatalf("missing key errors: %+v", data)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/credits" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"usage_monthly":1}}`))
	}))
	defer server.Close()
	data = (Client{Key: "test-key", HTTPClient: server.Client(), BaseURL: server.URL}).Read(context.Background())
	if data.Key.Error != "" || data.Credits.Error != "OpenRouter returned HTTP 403; /credits may require a management key" {
		t.Fatalf("permission errors: %+v", data)
	}
}
