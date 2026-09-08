package config

import (
	"errors"
	"os"
	"time"
)

const (
	DefaultAddress         = "127.0.0.1:8080"
	DefaultCodexExecutable = "codex"
	DefaultRefreshInterval = time.Minute
)

type Config struct {
	Address         string
	CodexExecutable string
	OpenRouterKey   string
	RefreshInterval time.Duration
}

func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

func LoadFrom(getenv func(string) string) (Config, error) {
	c := Config{
		Address:         getenv("GRYPHDASH_ADDR"),
		CodexExecutable: getenv("GRYPHDASH_CODEX_BIN"),
		OpenRouterKey:   getenv("OPENROUTER_API_KEY"),
		RefreshInterval: DefaultRefreshInterval,
	}
	if c.Address == "" {
		c.Address = DefaultAddress
	}
	if c.CodexExecutable == "" {
		c.CodexExecutable = DefaultCodexExecutable
	}
	if raw := getenv("GRYPHDASH_REFRESH_INTERVAL"); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil || interval < 30*time.Second {
			return Config{}, errors.New("GRYPHDASH_REFRESH_INTERVAL must be a duration of at least 30s")
		}
		c.RefreshInterval = interval
	}
	return c, nil
}
