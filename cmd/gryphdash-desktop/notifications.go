package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type wailsNotifier struct{}

func (wailsNotifier) Notify(ctx context.Context, title, body string) error {
	return wailsruntime.SendNotification(ctx, wailsruntime.NotificationOptions{Title: title, Body: body})
}
