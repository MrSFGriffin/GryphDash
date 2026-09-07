package main

import (
	"context"
	_ "embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web/dashboard.html
var dashboardHTML string

func newHandler(c *collector) http.Handler {
	page := template.Must(template.New("dashboard").Parse(dashboardHTML))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodHead {
			return
		}
		if err := page.Execute(w, buildDashboard(c.snapshot(), time.Now())); err != nil {
			log.Printf("render dashboard: %v", err)
		}
	})
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	interval := time.Minute
	if raw := os.Getenv("GRYPHDASH_REFRESH_INTERVAL"); raw != "" {
		var err error
		interval, err = time.ParseDuration(raw)
		if err != nil || interval < 30*time.Second {
			log.Fatal("GRYPHDASH_REFRESH_INTERVAL must be a duration of at least 30s")
		}
	}
	executable := os.Getenv("GRYPHDASH_CODEX_BIN")
	if executable == "" {
		executable = "codex"
	}
	c := &collector{executable: executable}
	done := make(chan struct{})
	go func() { defer close(done); c.run(ctx, interval) }()

	addr := os.Getenv("GRYPHDASH_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           newHandler(c),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("GryphDash listening on http://%s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server: %v", err)
	}
	stop()
	<-done
}
