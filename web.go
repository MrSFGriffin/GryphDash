package main

import (
	"context"
	"log"
	"net/http"

	"gryphdash/internal/app"
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
	runtime, err := app.New(app.Options{
		Collector: c,
		Handler:   webhandler.NewHandlerWithCatalogAndRepositoriesAndService(c, webassets.FS, catalog, repositories, service),
		Address:   cfg.Address,
		Interval:  cfg.RefreshInterval,
	})
	if err != nil {
		return err
	}
	log.Printf("GryphDash listening on http://%s", cfg.Address)
	return runtime.Run(ctx)
}
