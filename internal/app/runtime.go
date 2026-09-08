package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"gryphdash/internal/collector"
)

type Runtime struct {
	collector *collector.Collector
	server    *http.Server
	interval  time.Duration
	listen    func(network, address string) (net.Listener, error)
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

func (r *Runtime) Collector() *collector.Collector { return r.collector }

func (r *Runtime) Run(ctx context.Context) error {
	listen := r.listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp", r.server.Addr)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		r.collector.Run(runCtx, r.interval)
	}()
	go func() {
		<-runCtx.Done()
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
