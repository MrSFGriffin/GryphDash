package main

import (
	"context"
	"log"
	"net/http"
	"sync"

	"gryphdash/internal/app"
	collectorpkg "gryphdash/internal/collector"
	"gryphdash/internal/config"
	dashboardpkg "gryphdash/internal/dashboard"
	"gryphdash/internal/providerrepo"
	webhandler "gryphdash/internal/web"
	webassets "gryphdash/web"
)

func newHandler(c *collector) http.Handler {
	return webhandler.NewHandler(c, webassets.FS)
}

func newHandlerWithCatalog(c *collector, catalog dashboardpkg.WidgetCatalog) http.Handler {
	return webhandler.NewHandlerWithCatalog(c, webassets.FS, catalog)
}

func newHandlerWithCatalogAndRepositories(c *collector, catalog dashboardpkg.WidgetCatalog, repositories []providerrepo.Discovery) http.Handler {
	return webhandler.NewHandlerWithCatalogAndRepositoriesAndService(c, webassets.FS, catalog, repositories, nil)
}

func runWeb(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	c, catalog, repositories := newCollectorAndCatalog(cfg)
	service := newProviderService(cfg, repositories)
	var catalogMu sync.RWMutex
	if service != nil {
		service.SetProviderChangeCallback(func() {
			external, refreshedCatalog := discoverExternalProviders(cfg)
			readers := make([]collectorpkg.Reader, 0, len(external))
			for _, provider := range external {
				readers = append(readers, provider)
			}
			c.SetProviders(readers)
			catalogMu.Lock()
			catalog = refreshedCatalog
			catalogMu.Unlock()
			c.Refresh(context.Background())
		})
	}
	runtime, err := app.New(app.Options{
		Collector: c,
		Handler:   webhandler.NewHandlerWithCatalogSourceAndRepositoriesAndService(c, webassets.FS, func() dashboardpkg.WidgetCatalog { catalogMu.RLock(); defer catalogMu.RUnlock(); return catalog }, repositories, service),
		Address:   cfg.Address,
		Interval:  cfg.RefreshInterval,
	})
	if err != nil {
		return err
	}
	log.Printf("GryphDash listening on http://%s", cfg.Address)
	return runtime.Run(ctx)
}
