package main

import (
	"context"
	"errors"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gryphdash/internal/desktop"
)

// DesktopBridge is the deliberately small native API exposed to the Wails
// frontend. Browser sessions do not receive these bindings.
type DesktopBridge struct {
	controller    *desktop.Controller
	contextFn     func() context.Context
	settingsPath  string
	settingsMu    sync.RWMutex
	settings      desktop.Settings
	launchAtLogin desktop.LaunchAtLogin
	scopeMu       sync.RWMutex
	scope         map[string]bool
}

func (b *DesktopBridge) RefreshNow() { b.controller.Refresh(b.contextFn()) }
func (b *DesktopBridge) ShowWindow() { b.controller.Show(b.contextFn()) }
func (b *DesktopBridge) HideWindow() { b.controller.Hide(b.contextFn()) }
func (b *DesktopBridge) Quit()       { b.controller.Quit(b.contextFn()) }

func (b *DesktopBridge) CloseToTrayEnabled() bool { return b.controller.CloseToTrayEnabled() }

func (b *DesktopBridge) SetCloseToTray(enabled bool) error {
	b.controller.SetCloseToTray(enabled)
	b.settingsMu.Lock()
	b.settings.CloseToTray = enabled
	settings := b.settings
	b.settingsMu.Unlock()
	return b.saveSettings(settings)
}

func (b *DesktopBridge) LaunchAtLoginEnabled() (bool, error) {
	if b.launchAtLogin == nil {
		return false, errors.New("launch at login is unavailable")
	}
	return b.launchAtLogin.Enabled()
}

func (b *DesktopBridge) SetLaunchAtLogin(enabled bool) error {
	if b.launchAtLogin == nil {
		return errors.New("launch at login is unavailable")
	}
	if err := b.launchAtLogin.SetEnabled(enabled); err != nil {
		return err
	}
	b.settingsMu.Lock()
	b.settings.LaunchAtLogin = enabled
	settings := b.settings
	b.settingsMu.Unlock()
	return b.saveSettings(settings)
}

func (b *DesktopBridge) NotificationsAvailable() bool {
	return wailsruntime.IsNotificationAvailable(b.contextFn())
}

func (b *DesktopBridge) NotificationsEnabled() bool {
	b.settingsMu.RLock()
	defer b.settingsMu.RUnlock()
	return b.settings.NotificationsEnabled
}

func (b *DesktopBridge) SetNotificationsEnabled(enabled bool) error {
	b.settingsMu.Lock()
	b.settings.NotificationsEnabled = enabled
	settings := b.settings
	b.settingsMu.Unlock()
	return b.saveSettings(settings)
}

func (b *DesktopBridge) NotificationPreferences() desktop.NotificationPreferences {
	b.settingsMu.RLock()
	defer b.settingsMu.RUnlock()
	return b.settings.NotificationPreferences()
}

func (b *DesktopBridge) SetNotificationPreferences(preferences desktop.NotificationPreferences) error {
	b.settingsMu.Lock()
	b.settings.NotificationsEnabled = preferences.Enabled
	b.settings.NotifyFailures = preferences.Failures
	b.settings.NotifyRecovery = preferences.Recovery
	b.settings.NotifyStale = preferences.Stale
	settings := b.settings
	b.settingsMu.Unlock()
	return b.saveSettings(settings)
}

func (b *DesktopBridge) SetNotificationScope(sources []string) {
	scope := make(map[string]bool, len(sources))
	for _, source := range sources {
		scope[source] = true
	}
	b.scopeMu.Lock()
	b.scope = scope
	b.scopeMu.Unlock()
}

func (b *DesktopBridge) NotificationScope() map[string]bool {
	b.scopeMu.RLock()
	defer b.scopeMu.RUnlock()
	scope := make(map[string]bool, len(b.scope))
	for source := range b.scope {
		scope[source] = true
	}
	return scope
}

func (b *DesktopBridge) saveSettings(settings desktop.Settings) error {
	if b.settingsPath == "" {
		return errors.New("desktop settings are unavailable")
	}
	return desktop.SaveSettings(b.settingsPath, settings)
}
