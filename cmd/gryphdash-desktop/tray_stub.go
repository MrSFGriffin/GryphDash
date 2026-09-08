//go:build !desktop

package main

import (
	"context"

	"gryphdash/internal/desktop"
)

// The native tray backend is compiled for Wails desktop builds. Keeping a
// stub for ordinary Go tooling lets package-wide tests run without GTK or
// pkg-config installed.
func runTray([]byte, *desktop.Controller, func() context.Context, func()) {}
func stopTray()                                                           {}
