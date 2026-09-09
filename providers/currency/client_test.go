package currency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rate/EUR/USD" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"date":"2026-09-10","base":"EUR","quote":"USD","rate":1.17}`))
	}))
	defer server.Close()
	results := (Client{HTTPClient: server.Client(), BaseURL: server.URL, Pairs: []Pair{{Base: "EUR", Quote: "USD"}}}).Read(context.Background())
	result := results["currency/eur-usd"]
	if result.Error != "" || result.Data["rate"] != 1.17 {
		t.Fatalf("result = %+v", result)
	}
}

func TestParsePairs(t *testing.T) {
	got := ParsePairs("EUR/USD, EUR/GBP")
	if len(got) != 2 || got[1].Base != "EUR" || got[1].Quote != "GBP" {
		t.Fatalf("pairs = %+v", got)
	}
}
