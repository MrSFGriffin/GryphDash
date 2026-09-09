package desktop

import (
	"context"
	"time"

	"gryphdash/internal/collector"
)

type Notifier interface {
	Notify(context.Context, string, string) error
}

type NotificationMonitor struct {
	notifier         Notifier
	lastFailure      bool
	lastNotification time.Time
	minInterval      time.Duration
	staleAfter       time.Duration
	preferences      func() NotificationPreferences
	scope            func() map[string]bool
}

func NewNotificationMonitor(notifier Notifier, minInterval, staleAfter time.Duration, preferences func() NotificationPreferences, scope func() map[string]bool) *NotificationMonitor {
	return &NotificationMonitor{notifier: notifier, minInterval: minInterval, staleAfter: staleAfter, preferences: preferences, scope: scope}
}

func (m *NotificationMonitor) Observe(ctx context.Context, snapshot collector.Snapshot, now time.Time) {
	preferences := NotificationPreferences{Enabled: true, Failures: true, Recovery: true, Stale: true}
	if m.preferences != nil {
		preferences = m.preferences()
	}
	if !preferences.Enabled {
		return
	}
	visible := map[string]bool{}
	if m.scope != nil {
		visible = m.scope()
	}
	if len(visible) == 0 {
		return
	}
	failed := false
	for source, result := range snapshot.Results {
		if visible[source] && result.Error != "" {
			failed = true
			break
		}
	}
	stale := !snapshot.LastRefresh.IsZero() && m.staleAfter > 0 && now.Sub(snapshot.LastRefresh) >= m.staleAfter
	issue := failed || stale
	if issue && !m.lastFailure && stale && !failed && !preferences.Stale {
		return
	}
	if issue && !m.lastFailure && failed && !preferences.Failures {
		return
	}
	if !issue && m.lastFailure && !preferences.Recovery {
		m.lastFailure = false
		return
	}
	if issue == m.lastFailure && (!issue || now.Sub(m.lastNotification) < m.minInterval) {
		return
	}
	title, body := "GryphDash", "Provider data is unavailable."
	if issue {
		switch {
		case failed && stale:
			body = "Provider reads failed and displayed values may be stale."
		case failed:
			body = "One or more provider reads failed; displayed values may be stale."
		default:
			body = "Dashboard data has not refreshed recently; displayed values may be stale."
		}
		if m.lastFailure {
			title = "GryphDash still unavailable"
		} else {
			title = "GryphDash provider failure"
		}
	} else if m.lastFailure {
		title, body = "GryphDash recovered", "Provider data is available again."
	} else {
		return
	}
	if m.notifier == nil {
		return
	}
	if m.notifier.Notify(ctx, title, body) == nil {
		m.lastFailure = issue
		m.lastNotification = now
	}
}

func (m *NotificationMonitor) Run(ctx context.Context, snapshot func() collector.Snapshot, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		m.Observe(ctx, snapshot(), time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
