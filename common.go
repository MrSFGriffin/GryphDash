package main

import (
	"net/http"
	"time"

	collectorpkg "gryphdash/internal/collector"
	"gryphdash/internal/config"
)

func newCollector(cfg config.Config) *collector {
	return collectorpkg.New(collectorpkg.Options{
		CodexExecutable: cfg.CodexExecutable,
		OpenRouterKey:   cfg.OpenRouterKey,
		HTTPClient:      &http.Client{Timeout: 15 * time.Second},
	})
}
