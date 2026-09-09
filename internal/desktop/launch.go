package desktop

type LaunchAtLogin interface {
	Enabled() (bool, error)
	SetEnabled(bool) error
}
