package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var openRouterAPIBase = "https://openrouter.ai/api/v1"

func (c *collector) refreshOpenRouter(parent context.Context) {
	if c.openRouterKey == "" {
		c.setOpenRouterError("OPENROUTER_API_KEY is not configured")
		return
	}
	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	keyData, keyErr := c.openRouterRequest(ctx, client, "/key")
	creditsData, creditsErr := c.openRouterRequest(ctx, client, "/credits")
	c.mu.Lock()
	if keyErr != nil {
		c.state.OpenRouterKey.Error = keyErr.Error()
	} else {
		c.state.OpenRouterKey = result{Data: keyData, Updated: time.Now()}
	}
	if creditsErr != nil {
		c.state.OpenRouterCredits.Error = creditsErr.Error()
	} else {
		c.state.OpenRouterCredits = result{Data: creditsData, Updated: time.Now()}
	}
	c.mu.Unlock()
}

func (c *collector) openRouterRequest(ctx context.Context, client *http.Client, path string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterAPIBase+path, nil)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter request setup failed")
	}
	req.Header.Set("Authorization", "Bearer "+c.openRouterKey)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenRouter request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenRouter returned HTTP %d%s", resp.StatusCode, openRouterPermissionHint(path, resp.StatusCode))
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("OpenRouter returned invalid JSON")
	}
	if envelope.Data == nil {
		return nil, fmt.Errorf("OpenRouter returned no data")
	}
	return envelope.Data, nil
}

func openRouterPermissionHint(path string, status int) string {
	if path == "/credits" && (status == http.StatusForbidden || status == http.StatusUnauthorized) {
		return "; /credits may require a management key"
	}
	return ""
}

func (c *collector) setOpenRouterError(message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range []*result{&c.state.OpenRouterKey, &c.state.OpenRouterCredits} {
		r.Error = message
	}
}
