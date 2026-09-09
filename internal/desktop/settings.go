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
}

func DefaultSettings() Settings {
	return Settings{CloseToTray: true, NotificationsEnabled: true}
}

func SettingsPath(appName string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName, "settings.json"), nil
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
