package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"gryphdash/internal/collector"
	"gryphdash/internal/config"
)

type Runtime struct {
	mu        sync.Mutex
	collector *collector.Collector
	server    *http.Server
	interval  time.Duration
	listen    func(network, address string) (net.Listener, error)
	cancel    context.CancelFunc
	done      chan struct{}
	running   bool
}

type Options struct {
	Collector *collector.Collector
	Handler   http.Handler
	Address   string
	Interval  time.Duration
	Listen    func(network, address string) (net.Listener, error)
}

func New(options Options) (*Runtime, error) {
	if options.Collector == nil {
		return nil, errors.New("runtime collector is required")
	}
	if options.Handler == nil {
		return nil, errors.New("runtime handler is required")
	}
	if options.Address == "" {
		return nil, errors.New("runtime address is required")
	}
	if options.Interval <= 0 {
		return nil, errors.New("runtime refresh interval must be positive")
	}
	return &Runtime{
		collector: options.Collector,
		server: &http.Server{
			Addr:              options.Address,
			Handler:           options.Handler,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		interval: options.Interval,
		listen:   options.Listen,
	}, nil
}

func NewDesktop(options Options) (*Runtime, error) {
	if err := config.ValidateDesktopAddress(options.Address); err != nil {
		return nil, err
	}
	return New(options)
}

func (r *Runtime) Collector() *collector.Collector { return r.collector }

func (r *Runtime) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return errors.New("runtime is already running")
	}
	runningCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.done = make(chan struct{})
	r.running = true
	done := r.done
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		r.cancel = nil
		r.running = false
		close(done)
		r.mu.Unlock()
	}()

	listen := r.listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp", r.server.Addr)
	if err != nil {
		return fmt.Errorf("HTTP server cannot listen on %s: %w", r.server.Addr, err)
	}
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		r.collector.Run(runningCtx, r.interval)
	}()
	go func() {
		<-runningCtx.Done()
		shutdown, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = r.server.Shutdown(shutdown)
	}()

	err = r.server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		cancel()
		<-collectorDone
		return err
	}
	<-collectorDone
	return nil
}

// Shutdown requests an orderly stop and waits for the HTTP server, collector
// refresh loop, and any provider subprocesses to finish. It is safe to call
// more than once, including from a Wails shutdown callback and its caller.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	running := r.running
	r.mu.Unlock()
	if !running {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
