package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	c, err := LoadFrom(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != DefaultAddress || c.DesktopAddress != DefaultDesktopAddress || c.CodexExecutable != DefaultCodexExecutable || c.RefreshInterval != DefaultRefreshInterval {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	values := map[string]string{
		"GRYPHDASH_ADDR":             "127.0.0.1:9090",
		"GRYPHDASH_DESKTOP_ADDR":     "127.0.0.1:9091",
		"GRYPHDASH_CODEX_BIN":        "/opt/codex",
		"OPENROUTER_API_KEY":         "key",
		"GRYPHDASH_REFRESH_INTERVAL": "2m",
	}
	c, err := LoadFrom(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != values["GRYPHDASH_ADDR"] || c.DesktopAddress != values["GRYPHDASH_DESKTOP_ADDR"] || c.CodexExecutable != values["GRYPHDASH_CODEX_BIN"] || c.OpenRouterKey != values["OPENROUTER_API_KEY"] || c.RefreshInterval != 2*time.Minute {
		t.Fatalf("unexpected configuration: %+v", c)
	}
}

func TestLoadRejectsNonLoopbackDesktopAddress(t *testing.T) {
	_, err := LoadDesktopFrom(func(key string) string {
		if key == "GRYPHDASH_DESKTOP_ADDR" {
			return "0.0.0.0:8081"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDesktopAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8081", "127.0.0.1:65535"} {
		if err := ValidateDesktopAddress(address); err != nil {
			t.Errorf("%s: %v", address, err)
		}
	}
	for _, address := range []string{"localhost:8081", "0.0.0.0:8081", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1"} {
		if err := ValidateDesktopAddress(address); err == nil {
			t.Errorf("expected %s to be rejected", address)
		}
	}
}

func TestLoadRejectsShortRefreshInterval(t *testing.T) {
	_, err := LoadFrom(func(key string) string {
		if key == "GRYPHDASH_REFRESH_INTERVAL" {
			return "29s"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "at least 30s") {
		t.Fatalf("unexpected error: %v", err)
	}
}
