//go:build !linux && !windows && !darwin

package desktop

import "errors"

type unsupportedLaunchAtLogin struct{}

func NewLaunchAtLogin(string, string) (LaunchAtLogin, error) { return unsupportedLaunchAtLogin{}, nil }
func (unsupportedLaunchAtLogin) Enabled() (bool, error) {
	return false, errors.New("launch at login is not supported on this platform yet")
}
func (unsupportedLaunchAtLogin) SetEnabled(bool) error {
	return errors.New("launch at login is not supported on this platform yet")
}
