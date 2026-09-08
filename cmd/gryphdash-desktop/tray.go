//go:build desktop

package main

import (
	"context"

	"github.com/getlantern/systray"

	"gryphdash/internal/desktop"
)

func runTray(icon []byte, controller *desktop.Controller, contextFn func() context.Context, onExit func()) {
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
