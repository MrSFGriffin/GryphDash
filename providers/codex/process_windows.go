//go:build windows

package codex

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureCommand(cmd *exec.Cmd) {
	// Prevent provider subprocesses from opening a console when launched by
	// the Windows Wails desktop application.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
