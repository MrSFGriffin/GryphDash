//go:build darwin

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type darwinLaunchAtLogin struct {
	path, label, executable string
}

func NewLaunchAtLogin(name, executable string) (LaunchAtLogin, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return darwinLaunchAtLogin{filepath.Join(home, "Library", "LaunchAgents", name+".plist"), name, executable}, nil
}

func (l darwinLaunchAtLogin) Enabled() (bool, error) {
	_, err := os.Stat(l.path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (l darwinLaunchAtLogin) SetEnabled(enabled bool) error {
	if !enabled {
		if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return err
	}
	escape := func(value string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(value)
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, escape(l.label), escape(l.executable))
	return os.WriteFile(l.path, []byte(content), 0600)
}
