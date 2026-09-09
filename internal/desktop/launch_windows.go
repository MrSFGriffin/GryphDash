//go:build windows

package desktop

import (
	"golang.org/x/sys/windows/registry"
)

type windowsLaunchAtLogin struct {
	name, executable string
}

func NewLaunchAtLogin(name, executable string) (LaunchAtLogin, error) {
	return windowsLaunchAtLogin{name: name, executable: executable}, nil
}

func (l windowsLaunchAtLogin) Enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()
	_, _, err = key.GetStringValue(l.name)
	if err == registry.ErrNotExist {
		return false, nil
	}
	return err == nil, err
}

func (l windowsLaunchAtLogin) SetEnabled(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enabled {
		err := key.DeleteValue(l.name)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	return key.SetStringValue(l.name, l.executable)
}
