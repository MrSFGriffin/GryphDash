package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Settings struct {
	CloseToTray          bool `json:"closeToTray"`
	LaunchAtLogin        bool `json:"launchAtLogin"`
	NotificationsEnabled bool `json:"notificationsEnabled"`
	NotifyFailures       bool `json:"notifyFailures"`
	NotifyRecovery       bool `json:"notifyRecovery"`
	NotifyStale          bool `json:"notifyStale"`
}

type NotificationPreferences struct {
	Enabled  bool `json:"enabled"`
	Failures bool `json:"failures"`
	Recovery bool `json:"recovery"`
	Stale    bool `json:"stale"`
}

func DefaultSettings() Settings {
	return Settings{CloseToTray: true, NotificationsEnabled: true, NotifyFailures: true, NotifyRecovery: true, NotifyStale: true}
}

func (s Settings) NotificationPreferences() NotificationPreferences {
	return NotificationPreferences{Enabled: s.NotificationsEnabled, Failures: s.NotifyFailures, Recovery: s.NotifyRecovery, Stale: s.NotifyStale}
}

func SettingsPath(appName string) (string, error) {
	dir, err := ConfigurationDir(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func ConfigurationDir(appName string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName), nil
}

func LogsDir(appName string) (string, error) {
	dir, err := ConfigurationDir(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}

func LoadSettings(path string) (Settings, error) {
	settings := DefaultSettings()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultSettings(), err
	}
	return settings, nil
}

func SaveSettings(path string, settings Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}
