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
