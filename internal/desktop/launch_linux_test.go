//go:build linux

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxLaunchAtLogin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autostart", "gryphdash.desktop")
	launch := linuxLaunchAtLogin{path: path, name: "GryphDash", executable: "/opt/Gryph Dash/gryphdash"}

	enabled, err := launch.Enabled()
	if err != nil || enabled {
		t.Fatalf("initial launch-at-login state = %v, %v; want false, nil", enabled, err)
	}
	if err := launch.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `Exec="/opt/Gryph Dash/gryphdash"`) {
		t.Fatalf("autostart file has unexpected Exec line: %s", data)
	}
	enabled, err = launch.Enabled()
	if err != nil || !enabled {
		t.Fatalf("enabled launch-at-login state = %v, %v; want true, nil", enabled, err)
	}
	if err := launch.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	enabled, err = launch.Enabled()
	if err != nil || enabled {
		t.Fatalf("final launch-at-login state = %v, %v; want false, nil", enabled, err)
	}
}
