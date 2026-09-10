package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gryphdash/internal/desktop"
	"gryphdash/internal/providerrepo"
)

// DesktopBridge is the deliberately small native API exposed to the Wails
// frontend. Browser sessions do not receive these bindings.
type DesktopBridge struct {
	controller      *desktop.Controller
	contextFn       func() context.Context
	settingsPath    string
	settingsMu      sync.RWMutex
	settings        desktop.Settings
	launchAtLogin   desktop.LaunchAtLogin
	scopeMu         sync.RWMutex
	scope           map[string]bool
	providerService *providerrepo.Service
}

func (b *DesktopBridge) RefreshNow() { b.controller.Refresh(b.contextFn()) }
func (b *DesktopBridge) ProviderRepositories() []providerrepo.Discovery {
	if b.providerService == nil {
		return nil
	}
	return b.providerService.Repositories()
}
func (b *DesktopBridge) ProviderStatuses() []providerrepo.ProviderStatus {
	if b.providerService == nil {
		return nil
	}
	return b.providerService.Statuses()
}
func (b *DesktopBridge) AddProviderRepository(url string) error {
	if b.providerService == nil {
		return errors.New("provider management is unavailable")
	}
	_, err := b.providerService.AddRepository(url)
	return err
}
func (b *DesktopBridge) SetProviderRepositoryEnabled(url string, enabled bool) error {
	if b.providerService == nil {
		return errors.New("provider management is unavailable")
	}
	_, err := b.providerService.SetRepositoryEnabled(url, enabled)
	return err
}
func (b *DesktopBridge) RemoveProviderRepository(url string) error {
	if b.providerService == nil {
		return errors.New("provider management is unavailable")
	}
	_, err := b.providerService.RemoveRepository(url)
	return err
}
func (b *DesktopBridge) InstallProvider(url, providerID string) error {
	if b.providerService == nil {
		return errors.New("provider management is unavailable")
	}
	return b.providerService.Install(context.Background(), url, providerID)
}
func (b *DesktopBridge) UpdateProvider(url, providerID string) error {
	if b.providerService == nil {
		return errors.New("provider management is unavailable")
	}
	return b.providerService.Update(context.Background(), url, providerID)
}
func (b *DesktopBridge) RemoveProvider(url, providerID, version string) error {
	if b.providerService == nil {
		return errors.New("provider management is unavailable")
	}
	return b.providerService.RemoveProvider(url, providerID, version)
}
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

func (b *DesktopBridge) OpenConfiguration() error {
	if b.settingsPath == "" {
		return errors.New("desktop configuration is unavailable")
	}
	return desktop.OpenDirectory(filepath.Dir(b.settingsPath))
}

func (b *DesktopBridge) OpenLogs() error {
	logs, err := desktop.LogsDir("gryphdash")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(logs, 0700); err != nil {
		return err
	}
	return desktop.OpenDirectory(logs)
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
