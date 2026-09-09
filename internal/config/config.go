package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	DefaultAddress         = "127.0.0.1:8080"
	DefaultDesktopAddress  = "127.0.0.1:8081"
	DefaultCodexExecutable = "codex"
	DefaultRefreshInterval = time.Minute
)

type Config struct {
	Address           string
	DesktopAddress    string
	CodexExecutable   string
	OpenRouterKey     string
	ProviderDirectory string
	RefreshInterval   time.Duration
}

func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

func LoadDesktop() (Config, error) {
	return LoadDesktopFrom(os.Getenv)
}

func LoadDesktopFrom(getenv func(string) string) (Config, error) {
	c, err := LoadFrom(getenv)
	if err != nil {
		return Config{}, err
	}
	if err := ValidateDesktopAddress(c.DesktopAddress); err != nil {
		return Config{}, err
	}
	return c, nil
}

func LoadFrom(getenv func(string) string) (Config, error) {
	c := Config{
		Address:           getenv("GRYPHDASH_ADDR"),
		DesktopAddress:    getenv("GRYPHDASH_DESKTOP_ADDR"),
		CodexExecutable:   getenv("GRYPHDASH_CODEX_BIN"),
		OpenRouterKey:     getenv("OPENROUTER_API_KEY"),
		ProviderDirectory: getenv("GRYPHDASH_PROVIDER_DIR"),
		RefreshInterval:   DefaultRefreshInterval,
	}
	if c.Address == "" {
		c.Address = DefaultAddress
	}
	if c.DesktopAddress == "" {
		c.DesktopAddress = DefaultDesktopAddress
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

func ValidateDesktopAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("desktop address %q must be a loopback host and TCP port: %w", address, err)
	}
	if host != "127.0.0.1" {
		return fmt.Errorf("desktop address %q must bind to 127.0.0.1", address)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("desktop address %q must use a TCP port from 1 to 65535", address)
	}
	return nil
}
