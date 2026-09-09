//go:build !linux && !darwin && !windows

package desktop

func OpenDirectory(string) error { return ErrOpenDirectoryUnsupported }
