package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"gryphdash/internal/collector"
)

type emptyReader struct{}

func (emptyReader) Name() string { return "empty" }
func (emptyReader) Read(context.Context) map[string]collector.Result {
	return nil
}

type cancellationReader struct {
	started chan struct{}
}

func (r cancellationReader) Name() string { return "cancellation" }
func (r cancellationReader) Read(ctx context.Context) map[string]collector.Result {
	close(r.started)
	<-ctx.Done()
	return nil
}

func TestRuntimeStartsAndShutsDown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	collectorInstance := collector.New(collector.Options{
		Providers: []collector.Reader{emptyReader{}},
	})
	runtime, err := New(Options{
		Collector: collectorInstance,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "ok")
		}),
		Address:  listener.Addr().String(),
		Interval: time.Millisecond,
		Listen: func(string, string) (net.Listener, error) {
			return listener, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()

	client := &http.Client{Timeout: time.Second}
	var response *http.Response
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		response, err = client.Get("http://" + listener.Addr().String())
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime did not shut down")
	}
}

func TestRuntimeShutdownWaitsForCollector(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	started := make(chan struct{})
	runtime, err := New(Options{
		Collector: collector.New(collector.Options{Providers: []collector.Reader{cancellationReader{started}}}),
		Handler:   http.NotFoundHandler(),
		Address:   listener.Addr().String(),
		Interval:  time.Hour,
		Listen: func(string, string) (net.Listener, error) {
			return listener, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan error, 1)
	go func() { runDone <- runtime.Run(context.Background()) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("collector did not start")
	}

	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := runtime.Shutdown(shutdownContext); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime did not finish after shutdown")
	}
}

func TestRuntimeRequiresCoreOptions(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected missing collector error")
	}
	if _, err := New(Options{Collector: collector.New(collector.Options{})}); err == nil {
		t.Fatal("expected missing handler error")
	}
}

func TestDesktopRuntimeRejectsNonLoopbackAddress(t *testing.T) {
	_, err := NewDesktop(Options{
		Collector: collector.New(collector.Options{}),
		Handler:   http.NotFoundHandler(),
		Address:   "0.0.0.0:8081",
		Interval:  time.Minute,
	})
	if err == nil {
		t.Fatal("expected non-loopback address to be rejected")
	}
}

func TestDesktopRuntimeReportsPortConflict(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	runtime, err := NewDesktop(Options{
		Collector: collector.New(collector.Options{}),
		Handler:   http.NotFoundHandler(),
		Address:   listener.Addr().String(),
		Interval:  time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Run(context.Background()); err == nil {
		t.Fatal("expected port conflict")
	} else if !strings.Contains(err.Error(), "HTTP server cannot listen") {
		t.Fatalf("unexpected error: %v", err)
	}
}
