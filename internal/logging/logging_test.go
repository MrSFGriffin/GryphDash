package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"
)

func TestParseLevel(t *testing.T) {
	for input, want := range map[string]slog.Level{"debug": slog.LevelDebug, "warning": slog.LevelWarn, "error": slog.LevelError, "": slog.LevelInfo} {
		if got := parseLevel(input); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestRotateKeepsRecentBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gryphdash.log")
	if err := os.WriteFile(path, make([]byte, maxLogSize), 0600); err != nil {
		t.Fatal(err)
	}
	if err := rotate(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated log missing: %v", err)
	}
}

func TestSetupWritesStartupEntry(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("APPDATA", config)
	t.Setenv("HOME", config)
	t.Setenv("GRYPHDASH_LOG_LEVEL", "info")
	previousLogger, previousOutput := slog.Default(), log.Writer()
	previousFlags, previousPrefix := log.Flags(), log.Prefix()
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})
	// A GUI executable can have an unusable stderr handle. Reproduce that
	// without starting Wails or contacting any providers.
	console, err := os.CreateTemp(t.TempDir(), "closed-console")
	if err != nil {
		t.Fatal(err)
	}
	if err := console.Close(); err != nil {
		t.Fatal(err)
	}
	previousStderr := os.Stderr
	os.Stderr = console
	t.Cleanup(func() { os.Stderr = previousStderr })
	closeLogs, err := Setup("gryphdash-test")
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("collector refresh completed", "providers", 2)
	log.Print("standard logger entry")
	if err := closeLogs(); err != nil {
		t.Fatal(err)
	}
	directory, err := Directory("gryphdash-test")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "gryphdash-test.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"logging initialized", "collector refresh completed", "standard logger entry"} {
		if !strings.Contains(string(data), entry) {
			t.Errorf("log missing %q with unavailable stderr: %s", entry, data)
		}
	}
}
