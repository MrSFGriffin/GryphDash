//go:build linux

package desktop

import "os/exec"

func OpenDirectory(path string) error { return exec.Command("xdg-open", path).Start() }
