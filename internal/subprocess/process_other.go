//go:build !windows

package subprocess

import "os/exec"

func configureCommand(*exec.Cmd) {}
