package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Data    map[string]any
	Updated time.Time
	Error   string
}
type Data struct{ Key, Credits Result }
type Client struct {
	Key        string
	HTTPClient *http.Client
	BaseURL    string
}

const DefaultBaseURL = "https://openrouter.ai/api/v1"

var BaseURLOverride = DefaultBaseURL

func (c Client) Read(parent context.Context) Data {
	out := Data{}
	if strings.TrimSpace(c.Key) == "" {
		out.Key.Error = "OPENROUTER_API_KEY is not configured"
		out.Credits.Error = out.Key.Error
		return out
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	key, err := c.request(ctx, "/key")
	if err != nil {
		out.Key.Error = err.Error()
	} else {
		out.Key = Result{Data: key, Updated: time.Now()}
	}
	credits, err := c.request(ctx, "/credits")
	if err != nil {
		out.Credits.Error = err.Error()
	} else {
		out.Credits = Result{Data: credits, Updated: time.Now()}
	}
	return out
}
func (c Client) request(ctx context.Context, path string) (map[string]any, error) {
	if c.BaseURL == "" {
		c.BaseURL = BaseURLOverride
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if path == "/credits" && (response.StatusCode == 401 || response.StatusCode == 403) {
			return nil, fmt.Errorf("OpenRouter returned HTTP %d; /credits may require a management key", response.StatusCode)
		}
		return nil, fmt.Errorf("OpenRouter request failed: HTTP %s", response.Status)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("OpenRouter response invalid: %w", err)
	}
	return envelope.Data, nil
}
