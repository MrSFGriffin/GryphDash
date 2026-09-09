package desktop

import (
	"context"
	"testing"
	"time"

	"gryphdash/internal/collector"
)

type recordingNotifier struct{ titles []string }

func (n *recordingNotifier) Notify(context.Context, string, string) error {
	n.titles = append(n.titles, "notification")
	return nil
}

func TestNotificationMonitorTransitionsAndRateLimits(t *testing.T) {
	n := &recordingNotifier{}
	m := NewNotificationMonitor(n, time.Minute, 5*time.Minute, nil, func() map[string]bool { return map[string]bool{"codex/usage": true} })
	now := time.Unix(100, 0)
	failure := collector.Snapshot{Results: map[string]collector.Result{"codex/usage": {Error: "failed"}}}
	m.Observe(context.Background(), failure, now)
	m.Observe(context.Background(), failure, now.Add(time.Second))
	m.Observe(context.Background(), failure, now.Add(time.Minute))
	m.Observe(context.Background(), collector.Snapshot{}, now.Add(2*time.Minute))
	if len(n.titles) != 3 {
		t.Fatalf("got %d notifications, want failure, reminder, recovery", len(n.titles))
	}
}

func TestNotificationMonitorReportsStaleData(t *testing.T) {
	n := &recordingNotifier{}
	m := NewNotificationMonitor(n, time.Hour, 5*time.Minute, nil, func() map[string]bool { return map[string]bool{"codex/usage": true} })
	now := time.Unix(1000, 0)
	m.Observe(context.Background(), collector.Snapshot{LastRefresh: now.Add(-6 * time.Minute)}, now)
	if len(n.titles) != 1 {
		t.Fatalf("got %d notifications, want one stale-data notification", len(n.titles))
	}
	m.Observe(context.Background(), collector.Snapshot{LastRefresh: now}, now.Add(time.Second))
	if len(n.titles) != 2 {
		t.Fatalf("got %d notifications, want stale recovery notification", len(n.titles))
	}
}

func TestNotificationMonitorIgnoresFailuresOutsideBoardScope(t *testing.T) {
	n := &recordingNotifier{}
	m := NewNotificationMonitor(n, time.Minute, 5*time.Minute, nil, func() map[string]bool { return map[string]bool{"codex/usage": true} })
	m.Observe(context.Background(), collector.Snapshot{Results: map[string]collector.Result{"codex/account": {Error: "failed"}}}, time.Unix(100, 0))
	if len(n.titles) != 0 {
		t.Fatal("unexpected notification for a provider without a displayed widget")
	}
}
