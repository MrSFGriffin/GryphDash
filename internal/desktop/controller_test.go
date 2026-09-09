package desktop

import (
	"context"
	"testing"
)

type fakeWindow struct{ shown, hidden, quit int }

func (w *fakeWindow) Show(context.Context) { w.shown++ }
func (w *fakeWindow) Hide(context.Context) { w.hidden++ }
func (w *fakeWindow) Quit(context.Context) { w.quit++ }

func TestControllerClosesToTrayAndExplicitQuitCloses(t *testing.T) {
	window := &fakeWindow{}
	controller := NewController(window, true, Actions{})

	if !controller.BeforeClose(context.Background()) {
		t.Fatal("expected normal close to be prevented")
	}
	if window.hidden != 1 || window.quit != 0 {
		t.Fatalf("unexpected window calls after close: %+v", window)
	}

	controller.Quit(context.Background())
	if window.quit != 1 {
		t.Fatalf("expected explicit quit to close window: %+v", window)
	}
	if controller.BeforeClose(context.Background()) {
		t.Fatal("expected explicit quit to allow close")
	}
}

func TestControllerDispatchesHide(t *testing.T) {
	window := &fakeWindow{}
	controller := NewController(window, true, Actions{})
	controller.Hide(context.Background())
	if window.hidden != 1 {
		t.Fatalf("expected window to hide: %+v", window)
	}
}

func TestControllerCanDisableCloseToTray(t *testing.T) {
	window := &fakeWindow{}
	controller := NewController(window, false, Actions{})
	if controller.BeforeClose(context.Background()) {
		t.Fatal("expected close-to-tray to be disabled")
	}
	if window.hidden != 0 {
		t.Fatal("window should not be hidden when close-to-tray is disabled")
	}
}

func TestControllerDispatchesActions(t *testing.T) {
	window := &fakeWindow{}
	var shown, refreshed, quit bool
	controller := NewController(window, true, Actions{
		Show:    func(context.Context) { shown = true },
		Refresh: func(context.Context) { refreshed = true },
		Quit:    func(context.Context) { quit = true },
	})
	ctx := context.Background()
	controller.Show(ctx)
	controller.Refresh(ctx)
	controller.Quit(ctx)
	if !shown || !refreshed || !quit {
		t.Fatalf("actions were not dispatched: show=%v refresh=%v quit=%v", shown, refreshed, quit)
	}
}
