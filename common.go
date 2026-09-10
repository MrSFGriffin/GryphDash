package main

import (
	"context"
	"log"
	"net/http"
	"time"

	collectorpkg "gryphdash/internal/collector"
	"gryphdash/internal/config"
	dashboardpkg "gryphdash/internal/dashboard"
	"gryphdash/internal/providerrepo"
	"gryphdash/internal/subprocess"
)

func newCollector(cfg config.Config) *collector {
	c, _, _ := newCollectorAndCatalog(cfg)
	return c
}

func newCollectorAndCatalog(cfg config.Config) (*collector, dashboardpkg.WidgetCatalog, []providerrepo.Discovery) {
	providers := []collectorpkg.Reader{}
	external, err := subprocess.Discover(cfg.ProviderDirectory)
	if err != nil {
		log.Printf("external provider discovery: %v", err)
	} else {
		for _, provider := range external {
			providers = append(providers, provider)
		}
	}
	discoveries := discoverProviderRepositories()
	return collectorpkg.New(collectorpkg.Options{Providers: providers}), subprocess.Catalog(context.Background(), external), discoveries
}

func discoverProviderRepositories() []providerrepo.Discovery {
	settingsPath, err := providerrepo.SettingsPath("gryphdash")
	if err != nil {
		log.Printf("provider repository settings path: %v", err)
		return nil
	}
	settings, err := providerrepo.LoadSettings(settingsPath)
	if err != nil {
		log.Printf("provider repository settings: %v", err)
		settings = providerrepo.DefaultSettings()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return providerrepo.DiscoverConfigured(ctx, settings, &http.Client{Timeout: 10 * time.Second}, nil)
}
