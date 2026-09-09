package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gryphdash/internal/metrics"
)

type Result = metrics.Result
type Snapshot = metrics.Snapshot

// Reader is the collector's provider-neutral boundary. A reader returns
// already-namespaced source IDs, so the collector never interprets provider
// result names or response shapes.
type Reader interface {
	Name() string
	Read(context.Context) map[string]Result
}

type Options struct {
	Providers []Reader
}

type Collector struct {
	mu        sync.RWMutex
	state     Snapshot
	providers []Reader
}

func New(options Options) *Collector {
	return &Collector{providers: options.Providers, state: Snapshot{Results: map[string]Result{}}}
}

func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	results := make(map[string]Result, len(c.state.Results))
	for source, result := range c.state.Results {
		results[source] = result
	}
	return Snapshot{Results: results, LastRefresh: c.state.LastRefresh}
}

func (c *Collector) SetSnapshot(snapshot Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if snapshot.Results == nil {
		snapshot.Results = map[string]Result{}
	}
	c.state = snapshot
}

func (c *Collector) Fail() {
	c.mu.Lock()
	defer c.mu.Unlock()
	message := "Provider data is unavailable."
	for source, result := range c.state.Results {
		result.Error = message
		c.state.Results[source] = result
	}
}

func (c *Collector) Refresh(parent context.Context) {
	started := time.Now()
	slog.Info("collector refresh started", "providers", len(c.providers))
	responses := make(chan map[string]Result, len(c.providers))
	for _, provider := range c.providers {
		go func(p Reader) {
			providerStarted := time.Now()
			results := map[string]Result{}
			defer func() {
				if recovered := recover(); recovered != nil {
					slog.Error("provider panicked", "provider", p.Name(), "duration", time.Since(providerStarted), "panic", recovered)
				}
				failures := 0
				for source, result := range results {
					if result.Error != "" {
						failures++
						slog.Warn("provider source failed", "provider", p.Name(), "source", source, "error", result.Error)
					}
				}
				slog.Info("provider read completed", "provider", p.Name(), "duration", time.Since(providerStarted), "sources", len(results), "failures", failures)
				responses <- results
			}()
			results = p.Read(parent)
		}(provider)
	}
	for range c.providers {
		c.applyResults(<-responses)
	}
	c.mu.Lock()
	c.state.LastRefresh = time.Now().UTC()
	c.mu.Unlock()
	slog.Info("collector refresh completed", "duration", time.Since(started), "providers", len(c.providers))
}

func (c *Collector) applyResults(results map[string]Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Results == nil {
		c.state.Results = map[string]Result{}
	}
	for source, result := range results {
		if previous, ok := c.state.Results[source]; ok && result.Error != "" && !previous.Updated.IsZero() {
			result.Data = previous.Data
			result.Updated = previous.Updated
		}
		c.state.Results[source] = result
	}
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
