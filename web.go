package main

import (
	"context"
	"log"
	"net/http"

	"gryphdash/internal/app"
	"gryphdash/internal/config"
	webhandler "gryphdash/internal/web"
	webassets "gryphdash/web"
)

func newHandler(c *collector) http.Handler {
	return webhandler.NewHandler(c, webassets.FS)
}

func runWeb(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	c := newCollector(cfg)
	runtime, err := app.New(app.Options{
		Collector: c,
		Handler:   newHandler(c),
		Address:   cfg.Address,
		Interval:  cfg.RefreshInterval,
	})
	if err != nil {
		return err
	}
	log.Printf("GryphDash listening on http://%s", cfg.Address)
	return runtime.Run(ctx)
}
