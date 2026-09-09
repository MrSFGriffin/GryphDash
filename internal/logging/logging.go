package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxLogSize = 5 << 20
	logBackups = 3
)

func Directory(appName string) (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, appName, "logs"), nil
}

func Setup(appName string) (func() error, error) {
	directory, err := Directory(appName)
	if err != nil {
		return func() error { return nil }, err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return func() error { return nil }, err
	}
	path := filepath.Join(directory, appName+".log")
	if err := rotate(path); err != nil {
		return func() error { return nil }, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return func() error { return nil }, err
	}
	writer := fileFirstWriter{file: file, console: os.Stderr}
	level := new(slog.LevelVar)
	level.Set(parseLevel(os.Getenv("GRYPHDASH_LOG_LEVEL")))
	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	log.SetOutput(writer)
	// Keep the startup location visible even if the configured slog level hides
	// informational events.
	log.Printf("logging initialized path=%s level=%s", path, level.Level().String())
	slog.Info("logging initialized", "path", path, "level", level.Level().String())
	return file.Close, nil
}

// Console output is best effort: Windows GUI executables may have no usable
// stderr handle. File errors must still propagate to the caller.
type fileFirstWriter struct {
	file    io.Writer
	console io.Writer
}

func (w fileFirstWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if w.console != nil {
		_, _ = w.console.Write(p)
	}
	return n, err
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func rotate(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || info.Size() < maxLogSize {
		return err
	}
	for i := logBackups; i >= 1; i-- {
		old := path + "." + strconv.Itoa(i)
		if i == logBackups {
			if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.Rename(path+"."+strconv.Itoa(i), path+"."+strconv.Itoa(i+1)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(path, path+".1")
}
