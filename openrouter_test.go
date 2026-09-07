package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRefreshOpenRouter(t *testing.T) {
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
	previous := openRouterAPIBase
	openRouterAPIBase = server.URL
	defer func() { openRouterAPIBase = previous }()
	c := &collector{openRouterKey: "test-key", httpClient: server.Client()}
	c.refreshOpenRouter(context.Background())
	s := c.snapshot()
	if s.OpenRouterKey.Error != "" || s.OpenRouterCredits.Error != "" {
		t.Fatalf("unexpected errors: %+v %+v", s.OpenRouterKey, s.OpenRouterCredits)
	}
	if got := s.OpenRouterKey.Data["usage_monthly"]; got != float64(12.5) {
		t.Fatalf("monthly usage: %v", got)
	}
	if got := s.OpenRouterCredits.Data["total_credits"]; got != float64(100) {
		t.Fatalf("credits: %v", got)
	}
	if len(gotAuth) != 2 || gotAuth[0] != "Bearer test-key" || gotAuth[1] != "Bearer test-key" {
		t.Fatalf("authorization headers: %v", gotAuth)
	}
	if got := timestamp(s.OpenRouterKey.Data["expires_at"]); got != "2027-12-31 23:59 UTC" {
		t.Fatalf("ISO timestamp: %q", got)
	}
}

func TestOpenRouterErrorsAndMissingKey(t *testing.T) {
	c := &collector{}
	c.refreshOpenRouter(context.Background())
	s := c.snapshot()
	if s.OpenRouterKey.Error != "OPENROUTER_API_KEY is not configured" || s.OpenRouterCredits.Error == "" {
		t.Fatalf("missing key errors: %+v", s)
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
	previous := openRouterAPIBase
	openRouterAPIBase = server.URL
	defer func() { openRouterAPIBase = previous }()
	c = &collector{openRouterKey: "test-key", httpClient: server.Client()}
	c.refreshOpenRouter(context.Background())
	s = c.snapshot()
	if s.OpenRouterKey.Error != "" || s.OpenRouterCredits.Error != "OpenRouter returned HTTP 403; /credits may require a management key" {
		t.Fatalf("permission errors: %+v", s)
	}
}
