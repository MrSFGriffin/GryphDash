//go:build darwin

package desktop

import "os/exec"

func OpenDirectory(path string) error { return exec.Command("open", path).Start() }
