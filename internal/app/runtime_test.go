package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"gryphdash/internal/collector"
)

type emptyReader struct{}

func (emptyReader) Name() string { return "empty" }
func (emptyReader) Read(context.Context) map[string]collector.Result {
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

func TestRuntimeRequiresCoreOptions(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected missing collector error")
	}
	if _, err := New(Options{Collector: collector.New(collector.Options{})}); err == nil {
		t.Fatal("expected missing handler error")
	}
}
