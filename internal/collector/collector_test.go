package collector

import (
	"context"
	"testing"
	"time"
)

type blockingReader struct{ started chan struct{} }

func (blockingReader) Name() string { return "blocking" }
func (r blockingReader) Read(ctx context.Context) map[string]Result {
	close(r.started)
	<-ctx.Done()
	return nil
}

type immediateReader struct{ ready chan struct{} }

func (immediateReader) Name() string { return "immediate" }
func (r immediateReader) Read(context.Context) map[string]Result {
	close(r.ready)
	return map[string]Result{"account": {Data: map[string]any{"planType": "Plus"}, Updated: time.Now()}}
}

func TestProviderResultsBecomeVisibleIndividually(t *testing.T) {
	started := make(chan struct{})
	ready := make(chan struct{})
	c := New(Options{Providers: []Reader{blockingReader{started}, immediateReader{ready}}})
	refreshDone := make(chan struct{})
	refreshCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		c.Refresh(refreshCtx)
		close(refreshDone)
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("immediate provider did not finish")
	}
	deadline := time.Now().Add(time.Second)
	for c.Snapshot().Account.Updated.IsZero() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if c.Snapshot().Account.Updated.IsZero() {
		t.Fatal("finished provider result was not published before the blocked provider")
	}
	cancel()
	select {
	case <-refreshDone:
	case <-time.After(time.Second):
		t.Fatal("refresh did not stop after cancellation")
	}
}

func TestSnapshotRemainsReadableDuringRefresh(t *testing.T) {
	started := make(chan struct{})
	c := New(Options{Providers: []Reader{blockingReader{started}}})
	c.SetSnapshot(Snapshot{Account: Result{Updated: time.Now()}})

	refreshDone := make(chan struct{})
	refreshCtx, cancel := context.WithCancel(context.Background())
	go func() {
		c.Refresh(refreshCtx)
		close(refreshDone)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("provider did not start")
	}

	snapshotDone := make(chan struct{})
	go func() {
		_ = c.Snapshot()
		close(snapshotDone)
	}()
	select {
	case <-snapshotDone:
	case <-time.After(time.Second):
		t.Fatal("snapshot blocked during provider refresh")
	}
	cancel()
	select {
	case <-refreshDone:
	case <-time.After(time.Second):
		t.Fatal("refresh did not stop after cancellation")
	}
}
