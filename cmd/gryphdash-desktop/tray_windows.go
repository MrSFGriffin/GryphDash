//go:build desktop && windows

package main

import (
	"context"
	"runtime"

	"github.com/getlantern/systray"

	"gryphdash/internal/desktop"
)

// runTray owns the native window and message pump used by systray. Windows
// delivers tray notifications through the thread queue of the thread that
// created that window, so initialization and the message loop must remain on
// one OS thread for their full lifetime.
func runTray(icon []byte, controller *desktop.Controller, contextFn func() context.Context, onExit func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	systray.Run(func() {
		systray.SetIcon(gryphDashTrayIcon)
		systray.SetTooltip("GryphDash")
		show := systray.AddMenuItem("Show Dashboard", "Show the GryphDash window")
		refresh := systray.AddMenuItem("Refresh Now", "Refresh dashboard data")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Quit GryphDash")

		go func() {
			for {
				select {
				case <-show.ClickedCh:
					controller.Show(contextFn())
				case <-refresh.ClickedCh:
					controller.Refresh(contextFn())
				case <-quit.ClickedCh:
					controller.Quit(contextFn())
					return
				}
			}
		}()
	}, onExit)
}

func stopTray() { systray.Quit() }
