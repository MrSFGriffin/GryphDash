//go:build linux

package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type linuxLaunchAtLogin struct {
	path, name, executable string
}

func NewLaunchAtLogin(name, executable string) (LaunchAtLogin, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return linuxLaunchAtLogin{filepath.Join(configDir, "autostart", name+".desktop"), name, executable}, nil
}

func (l linuxLaunchAtLogin) Enabled() (bool, error) {
	_, err := os.Stat(l.path)
	return err == nil, func() error {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}()
}

func (l linuxLaunchAtLogin) SetEnabled(enabled bool) error {
	if !enabled {
		if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return err
	}
	content := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=%s\nExec=%s\nX-GNOME-Autostart-enabled=true\n", l.name, strconv.Quote(l.executable))
	return os.WriteFile(l.path, []byte(content), 0600)
}
