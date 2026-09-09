package desktop

import (
	"path/filepath"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	want := Settings{CloseToTray: false, LaunchAtLogin: true, NotificationsEnabled: false}
	if err := SaveSettings(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMissingSettingsUseDefaults(t *testing.T) {
	got, err := LoadSettings(t.TempDir() + "/missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSettings() {
		t.Fatalf("got %+v, want defaults", got)
	}
}

func TestConfigurationAndLogsDirectories(t *testing.T) {
	config, err := ConfigurationDir("gryphdash")
	if err != nil || filepath.Base(config) != "gryphdash" {
		t.Fatalf("configuration directory = %q, %v", config, err)
	}
	logs, err := LogsDir("gryphdash")
	if err != nil || filepath.Base(logs) != "logs" || filepath.Dir(logs) != config {
		t.Fatalf("logs directory = %q, %v", logs, err)
	}
}
