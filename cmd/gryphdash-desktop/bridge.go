package main

import (
	"context"

	"gryphdash/internal/desktop"
)

// DesktopBridge is the deliberately small native API exposed to the Wails
// frontend. Browser sessions do not receive these bindings.
type DesktopBridge struct {
	controller *desktop.Controller
	contextFn  func() context.Context
}

func (b *DesktopBridge) RefreshNow() { b.controller.Refresh(b.contextFn()) }
func (b *DesktopBridge) ShowWindow() { b.controller.Show(b.contextFn()) }
func (b *DesktopBridge) HideWindow() { b.controller.Hide(b.contextFn()) }
func (b *DesktopBridge) Quit()       { b.controller.Quit(b.contextFn()) }
