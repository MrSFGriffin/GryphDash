package main

import (
	"net/http"
	"time"

	collectorpkg "gryphdash/internal/collector"
	"gryphdash/internal/config"
	codexprovider "gryphdash/providers/codex"
	openrouterprovider "gryphdash/providers/openrouter"
)

func newCollector(cfg config.Config) *collector {
	return collectorpkg.New(collectorpkg.Options{
		Providers: []collectorpkg.Reader{
			codexprovider.NewAdapter(cfg.CodexExecutable),
			openrouterprovider.NewAdapter(cfg.OpenRouterKey, &http.Client{Timeout: 15 * time.Second}),
		},
	})
}
