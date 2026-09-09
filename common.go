package main

import (
	"log"
	"net/http"
	"time"

	collectorpkg "gryphdash/internal/collector"
	"gryphdash/internal/config"
	"gryphdash/internal/subprocess"
	codexprovider "gryphdash/providers/codex"
	openrouterprovider "gryphdash/providers/openrouter"
)

func newCollector(cfg config.Config) *collector {
	providers := []collectorpkg.Reader{
		codexprovider.NewAdapter(cfg.CodexExecutable),
		openrouterprovider.NewAdapter(cfg.OpenRouterKey, &http.Client{Timeout: 15 * time.Second}),
	}
	external, err := subprocess.Discover(cfg.ProviderDirectory)
	if err != nil {
		log.Printf("external provider discovery: %v", err)
	} else {
		for _, provider := range external {
			providers = append(providers, provider)
		}
	}
	return collectorpkg.New(collectorpkg.Options{Providers: providers})
}
