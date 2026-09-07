package main

import (
	"context"
	codexprovider "gryphdash/providers/codex"
	openrouterprovider "gryphdash/providers/openrouter"
	"net/http"
	"sync"
	"time"
)

type result = codexprovider.Result
type snapshot struct {
	Account, Limits, Usage           result
	OpenRouterKey, OpenRouterCredits result
}
type providerReader interface {
	Name() string
	Read(context.Context) map[string]result
}
type codexReader struct{ executable string }

func (p codexReader) Name() string { return "codex" }
func (p codexReader) Read(ctx context.Context) map[string]result {
	d := (codexprovider.Client{Executable: p.executable}).Read(ctx)
	return map[string]result{"account": result(d.Account), "limits": result(d.Limits), "usage": result(d.Usage)}
}

type openRouterReader struct {
	key    string
	client *http.Client
}

func (p openRouterReader) Name() string { return "openrouter" }
func (p openRouterReader) Read(ctx context.Context) map[string]result {
	d := (openrouterprovider.Client{Key: p.key, HTTPClient: p.client}).Read(ctx)
	return map[string]result{"openrouterKey": result(d.Key), "openrouterCredits": result(d.Credits)}
}

type collector struct {
	mu                        sync.RWMutex
	state                     snapshot
	executable, openRouterKey string
	httpClient                *http.Client
	providers                 []providerReader
}

func (c *collector) snapshot() snapshot { c.mu.RLock(); defer c.mu.RUnlock(); return c.state }
func (c *collector) fail() {
	c.mu.Lock()
	defer c.mu.Unlock()
	message := "Could not connect to Codex. Check that the CLI is installed and run codex login as the server user."
	for _, r := range []*result{&c.state.Account, &c.state.Limits, &c.state.Usage, &c.state.OpenRouterKey, &c.state.OpenRouterCredits} {
		r.Error = message
	}
}
func (c *collector) refresh(parent context.Context) {
	providers := c.providers
	if len(providers) == 0 {
		providers = []providerReader{codexReader{c.executable}, openRouterReader{c.openRouterKey, c.httpClient}}
	}
	type response struct{ results map[string]result }
	responses := make(chan response, len(providers))
	for _, provider := range providers {
		go func(p providerReader) { responses <- response{p.Read(parent)} }(provider)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for range providers {
		for name, src := range (<-responses).results {
			var dst *result
			switch name {
			case "account":
				dst = &c.state.Account
			case "limits":
				dst = &c.state.Limits
			case "usage":
				dst = &c.state.Usage
			case "openrouterKey":
				dst = &c.state.OpenRouterKey
			case "openrouterCredits":
				dst = &c.state.OpenRouterCredits
			}
			if dst == nil {
				continue
			}
			if src.Error != "" {
				dst.Error = src.Error
			} else {
				*dst = src
			}
		}
	}
}
func (c *collector) refreshOpenRouter(parent context.Context) {
	for name, src := range (openRouterReader{c.openRouterKey, c.httpClient}).Read(parent) {
		c.mu.Lock()
		if name == "openrouterKey" {
			if src.Error != "" {
				c.state.OpenRouterKey.Error = src.Error
			} else {
				c.state.OpenRouterKey = src
			}
		}
		if name == "openrouterCredits" {
			if src.Error != "" {
				c.state.OpenRouterCredits.Error = src.Error
			} else {
				c.state.OpenRouterCredits = src
			}
		}
		c.mu.Unlock()
	}
}
func (c *collector) run(ctx context.Context, interval time.Duration) {
	for {
		c.refresh(ctx)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
