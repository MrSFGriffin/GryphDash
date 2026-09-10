package main

import (
	"context"
	"log"
	"net/http"
	"time"

	collectorpkg "gryphdash/internal/collector"
	"gryphdash/internal/config"
	dashboardpkg "gryphdash/internal/dashboard"
	"gryphdash/internal/subprocess"
	openrouterprovider "gryphdash/providers/openrouter"
)

func newCollector(cfg config.Config) *collector {
	c, _ := newCollectorAndCatalog(cfg)
	return c
}

func newCollectorAndCatalog(cfg config.Config) (*collector, dashboardpkg.WidgetCatalog) {
	providers := []collectorpkg.Reader{openrouterprovider.NewAdapter(cfg.OpenRouterKey, &http.Client{Timeout: 15 * time.Second})}
	external, err := subprocess.Discover(cfg.ProviderDirectory)
	if err != nil {
		log.Printf("external provider discovery: %v", err)
	} else {
		for _, provider := range external {
			providers = append(providers, provider)
		}
	}
	return collectorpkg.New(collectorpkg.Options{Providers: providers}), subprocess.Catalog(context.Background(), external)
}
