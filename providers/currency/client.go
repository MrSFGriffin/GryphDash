package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gryphdash/internal/metrics"
)

const DefaultBaseURL = "https://api.frankfurter.dev/v2"

type Pair struct {
	Base  string
	Quote string
}

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	Pairs      []Pair
}

func (c Client) Read(ctx context.Context) map[string]metrics.Result {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	results := make(map[string]metrics.Result, len(c.Pairs))
	for _, pair := range c.Pairs {
		key := sourceName(pair)
		result, err := c.readPair(ctx, client, baseURL, pair)
		if err != nil {
			results[key] = metrics.Result{Error: err.Error()}
		} else {
			results[key] = result
		}
	}
	return results
}

func (c Client) readPair(parent context.Context, client *http.Client, baseURL string, pair Pair) (metrics.Result, error) {
	base, quote := strings.ToUpper(strings.TrimSpace(pair.Base)), strings.ToUpper(strings.TrimSpace(pair.Quote))
	if len(base) != 3 || len(quote) != 3 || base == quote {
		return metrics.Result{}, fmt.Errorf("invalid currency pair %q/%q", pair.Base, pair.Quote)
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	endpoint := baseURL + "/rate/" + url.PathEscape(base) + "/" + url.PathEscape(quote)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return metrics.Result{}, err
	}
	response, err := client.Do(req)
	if err != nil {
		return metrics.Result{}, fmt.Errorf("currency request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return metrics.Result{}, fmt.Errorf("currency request failed: HTTP %s", response.Status)
	}
	var data struct {
		Date  string  `json:"date"`
		Base  string  `json:"base"`
		Quote string  `json:"quote"`
		Rate  float64 `json:"rate"`
	}
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		return metrics.Result{}, fmt.Errorf("currency response invalid: %w", err)
	}
	if data.Base == "" || data.Quote == "" || data.Rate <= 0 {
		return metrics.Result{}, fmt.Errorf("currency response missing a valid rate")
	}
	return metrics.Result{Data: map[string]any{
		"base": data.Base, "quote": data.Quote, "rate": data.Rate, "date": data.Date,
	}, Updated: time.Now().UTC()}, nil
}

func sourceName(pair Pair) string {
	return "currency/" + strings.ToLower(strings.TrimSpace(pair.Base)) + "-" + strings.ToLower(strings.TrimSpace(pair.Quote))
}

func ParsePairs(raw string) []Pair {
	var pairs []Pair
	for _, item := range strings.Split(raw, ",") {
		parts := strings.Split(strings.TrimSpace(item), "/")
		if len(parts) == 2 {
			pairs = append(pairs, Pair{Base: strings.TrimSpace(parts[0]), Quote: strings.TrimSpace(parts[1])})
		}
	}
	return pairs
}
