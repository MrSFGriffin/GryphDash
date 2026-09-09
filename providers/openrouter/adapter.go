package openrouter

import (
	"context"
	"net/http"

	"gryphdash/internal/metrics"
)

type Adapter struct {
	key    string
	client *http.Client
}

func NewAdapter(key string, client *http.Client) Adapter {
	return Adapter{key: key, client: client}
}

func (Adapter) Name() string { return "openrouter" }

func (a Adapter) Read(ctx context.Context) map[string]metrics.Result {
	data := (Client{Key: a.key, HTTPClient: a.client}).Read(ctx)
	return map[string]metrics.Result{
		"openrouter/key":     {Data: data.Key.Data, Updated: data.Key.Updated, Error: data.Key.Error},
		"openrouter/credits": {Data: data.Credits.Data, Updated: data.Credits.Updated, Error: data.Credits.Error},
	}
}
