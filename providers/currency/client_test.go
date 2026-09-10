package currency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response string
		switch r.URL.Path {
		case "/rate/EUR/USD":
			response = `{"date":"2026-09-10","base":"EUR","quote":"USD","rate":1.17}`
		case "/rate/GBP/EUR":
			response = `{"date":"2026-09-10","base":"GBP","quote":"EUR","rate":1.15}`
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	results := (Client{HTTPClient: server.Client(), BaseURL: server.URL, Pairs: []Pair{{Base: "EUR", Quote: "USD"}, {Base: "GBP", Quote: "EUR"}}}).Read(context.Background())
	result := results["currency/eur-usd"]
	if result.Error != "" || result.Data["rate"] != 1.17 {
		t.Fatalf("result = %+v", result)
	}
	result = results["currency/gbp-eur"]
	if result.Error != "" || result.Data["rate"] != 1.15 {
		t.Fatalf("GBP/EUR result = %+v", result)
	}
}

func TestParsePairs(t *testing.T) {
	got := ParsePairs("EUR/USD, EUR/GBP")
	if len(got) != 2 || got[1].Base != "EUR" || got[1].Quote != "GBP" {
		t.Fatalf("pairs = %+v", got)
	}
}
