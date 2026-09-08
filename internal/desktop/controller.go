// Package desktop contains the platform-neutral desktop window behavior.
package desktop

import (
	"context"
	"sync"
)

// Window is the small part of the Wails window API needed by the controller.
type Window interface {
	Show(context.Context)
	Hide(context.Context)
	Quit(context.Context)
}

// Actions are the operations exposed by the tray and native application menu.
type Actions struct {
	Show    func(context.Context)
	Refresh func(context.Context)
	Quit    func(context.Context)
}

// Controller owns close-to-tray state and dispatches native actions. A normal
// window close is prevented and hides the window; only an explicit Quit is
// allowed to close the Wails application.
type Controller struct {
	mu          sync.Mutex
	window      Window
	closeToTray bool
	quitting    bool
	actions     Actions
}

func NewController(window Window, closeToTray bool, actions Actions) *Controller {
	return &Controller{window: window, closeToTray: closeToTray, actions: actions}
}

// BeforeClose reports whether Wails should prevent closing the application.
func (c *Controller) BeforeClose(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closeToTray || c.quitting {
		return false
	}
	c.window.Hide(ctx)
	return true
}

func (c *Controller) Show(ctx context.Context) {
	if c.actions.Show != nil {
		c.actions.Show(ctx)
		return
	}
	c.window.Show(ctx)
}

func (c *Controller) Refresh(ctx context.Context) {
	if c.actions.Refresh != nil {
		c.actions.Refresh(ctx)
	}
}

func (c *Controller) Quit(ctx context.Context) {
	c.mu.Lock()
	c.quitting = true
	c.mu.Unlock()
	if c.actions.Quit != nil {
		c.actions.Quit(ctx)
		return
	}
	c.window.Quit(ctx)
}
