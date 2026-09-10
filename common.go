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
	external, catalog := discoverExternalProviders(cfg)
	providers := make([]collectorpkg.Reader, 0, len(external))
	for _, provider := range external {
		providers = append(providers, provider)
	}
	discoveries := discoverProviderRepositories()
	return collectorpkg.New(collectorpkg.Options{Providers: providers}), catalog, discoveries
}

func discoverExternalProviders(cfg config.Config) ([]subprocess.Provider, dashboardpkg.WidgetCatalog) {
	cacheDirectory := cfg.ProviderCacheDirectory
	if cacheDirectory == "" {
		cacheDirectory, _ = providerrepo.DefaultCacheDir("gryphdash")
	}
	external, err := subprocess.DiscoverWithManaged(cfg.ProviderDirectory, cacheDirectory)
	if err != nil {
		log.Printf("external provider discovery: %v", err)
	} else {
	}
	catalog, catalogErr := subprocess.CatalogWithError(context.Background(), external)
	if catalogErr != nil {
		log.Printf("external provider catalog rejected: %v", catalogErr)
	}
	return external, catalog
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

func newProviderService(cfg config.Config, repositories []providerrepo.Discovery) *providerrepo.Service {
	settingsPath, err := providerrepo.SettingsPath("gryphdash")
	if err != nil {
		return nil
	}
	cacheDirectory := cfg.ProviderCacheDirectory
	if cacheDirectory == "" {
		cacheDirectory, _ = providerrepo.DefaultCacheDir("gryphdash")
	}
	return providerrepo.NewService(settingsPath, cacheDirectory, repositories)
}
