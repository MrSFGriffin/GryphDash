package collector

import (
	"context"
	"net/http"
	"sync"
	"time"

	codexprovider "gryphdash/providers/codex"
	openrouterprovider "gryphdash/providers/openrouter"
)

type Result = codexprovider.Result

type Snapshot struct {
	Account, Limits, Usage           Result
	OpenRouterKey, OpenRouterCredits Result
	LastRefresh                      time.Time
}

type Reader interface {
	Name() string
	Read(context.Context) map[string]Result
}

type Options struct {
	CodexExecutable string
	OpenRouterKey   string
	HTTPClient      *http.Client
	Providers       []Reader
}

type codexReader struct{ executable string }

func (p codexReader) Name() string { return "codex" }
func (p codexReader) Read(ctx context.Context) map[string]Result {
	d := (codexprovider.Client{Executable: p.executable}).Read(ctx)
	return map[string]Result{"account": Result(d.Account), "limits": Result(d.Limits), "usage": Result(d.Usage)}
}

type openRouterReader struct {
	key    string
	client *http.Client
}

func (p openRouterReader) Name() string { return "openrouter" }
func (p openRouterReader) Read(ctx context.Context) map[string]Result {
	d := (openrouterprovider.Client{Key: p.key, HTTPClient: p.client}).Read(ctx)
	return map[string]Result{"openrouterKey": Result(d.Key), "openrouterCredits": Result(d.Credits)}
}

type Collector struct {
	mu                        sync.RWMutex
	state                     Snapshot
	executable, openRouterKey string
	httpClient                *http.Client
	providers                 []Reader
}

func New(options Options) *Collector {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Collector{
		executable:    options.CodexExecutable,
		openRouterKey: options.OpenRouterKey,
		httpClient:    httpClient,
		providers:     options.Providers,
	}
}

func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Collector) SetSnapshot(snapshot Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = snapshot
}

func (c *Collector) Fail() {
	c.mu.Lock()
	defer c.mu.Unlock()
	message := "Could not connect to Codex. Check that the CLI is installed and run codex login as the server user."
	for _, r := range []*Result{&c.state.Account, &c.state.Limits, &c.state.Usage, &c.state.OpenRouterKey, &c.state.OpenRouterCredits} {
		r.Error = message
	}
}

func (c *Collector) Refresh(parent context.Context) {
	providers := c.providers
	if len(providers) == 0 {
		providers = []Reader{codexReader{c.executable}, openRouterReader{c.openRouterKey, c.httpClient}}
	}
	type response struct{ results map[string]Result }
	responses := make(chan response, len(providers))
	for _, provider := range providers {
		go func(p Reader) { responses <- response{p.Read(parent)} }(provider)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for range providers {
		for name, src := range (<-responses).results {
			var dst *Result
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
	c.state.LastRefresh = time.Now().UTC()
}

func (c *Collector) Run(ctx context.Context, interval time.Duration) {
	for {
		c.Refresh(ctx)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
