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
	enabled          func() bool
}

func NewNotificationMonitor(notifier Notifier, minInterval, staleAfter time.Duration, enabled func() bool) *NotificationMonitor {
	return &NotificationMonitor{notifier: notifier, minInterval: minInterval, staleAfter: staleAfter, enabled: enabled}
}

func (m *NotificationMonitor) Observe(ctx context.Context, snapshot collector.Snapshot, now time.Time) {
	if m.enabled != nil && !m.enabled() {
		return
	}
	failed := snapshot.Account.Error != "" || snapshot.Limits.Error != "" || snapshot.Usage.Error != "" || snapshot.OpenRouterKey.Error != "" || snapshot.OpenRouterCredits.Error != ""
	stale := !snapshot.LastRefresh.IsZero() && m.staleAfter > 0 && now.Sub(snapshot.LastRefresh) >= m.staleAfter
	issue := failed || stale
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
