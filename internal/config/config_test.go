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
	if c.Address != DefaultAddress || c.CodexExecutable != DefaultCodexExecutable || c.RefreshInterval != DefaultRefreshInterval {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	values := map[string]string{
		"GRYPHDASH_ADDR":             "127.0.0.1:9090",
		"GRYPHDASH_CODEX_BIN":        "/opt/codex",
		"OPENROUTER_API_KEY":         "key",
		"GRYPHDASH_REFRESH_INTERVAL": "2m",
	}
	c, err := LoadFrom(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != values["GRYPHDASH_ADDR"] || c.CodexExecutable != values["GRYPHDASH_CODEX_BIN"] || c.OpenRouterKey != values["OPENROUTER_API_KEY"] || c.RefreshInterval != 2*time.Minute {
		t.Fatalf("unexpected configuration: %+v", c)
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
