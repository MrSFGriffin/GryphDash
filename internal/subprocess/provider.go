// Package subprocess adapts an external provider executable to the collector.
package subprocess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gryphdash/internal/metrics"
	"gryphdash/internal/providerprotocol"
)

const defaultTimeout = 30 * time.Second

type Provider struct {
	name       string
	executable string
	args       []string
	timeout    time.Duration
}

func New(name, executable string, args ...string) Provider {
	return Provider{name: name, executable: executable, args: args, timeout: defaultTimeout}
}

// Discover returns providers whose executable names use the stable
// gryphdash-provider-<name> convention. When directory is empty, the directory
// containing the running executable is searched.
func Discover(directory string) ([]Provider, error) {
	if directory == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, nil
		}
		directory = filepath.Dir(executable)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	providers := make([]Provider, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), "gryphdash-provider-")
		if name == entry.Name() || name == "" {
			continue
		}
		name = strings.TrimSuffix(name, ".exe")
		for _, suffix := range []string{"-windows-amd64", "-linux-amd64", "-darwin-amd64", "-darwin-arm64"} {
			name = strings.TrimSuffix(name, suffix)
		}
		if name == "" {
			continue
		}
		providers = append(providers, New(name, filepath.Join(directory, entry.Name())))
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name() < providers[j].Name() })
	return providers, nil
}

func (p Provider) Name() string { return p.name }

func (p Provider) Read(parent context.Context) map[string]metrics.Result {
	ctx, cancel := context.WithTimeout(parent, p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.executable, p.args...)
	configureCommand(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		return p.failure(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return p.failure(err)
	}
	if err := cmd.Start(); err != nil {
		return p.failure(err)
	}

	requestErr := json.NewEncoder(input).Encode(providerprotocol.Request{Version: providerprotocol.Version, Method: "read"})
	_ = input.Close()
	if requestErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return p.failure(requestErr)
	}

	var response providerprotocol.Response
	decodeErr := json.NewDecoder(bufio.NewReader(output)).Decode(&response)
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return p.failure(ctx.Err())
	}
	if decodeErr != nil {
		return p.failure(fmt.Errorf("reading provider response: %w", decodeErr))
	}
	if waitErr != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return p.failure(fmt.Errorf("provider exited: %w: %s", waitErr, detail))
		}
		return p.failure(fmt.Errorf("provider exited: %w", waitErr))
	}
	if response.Version != providerprotocol.Version {
		return p.failure(fmt.Errorf("unsupported provider protocol version %d", response.Version))
	}
	if response.Provider != "" && response.Provider != p.name {
		slog.Warn("provider identity mismatch", "expected", p.name, "reported", response.Provider)
	}
	if response.Error != "" {
		return p.failure(fmt.Errorf("%s", response.Error))
	}
	if response.Results == nil {
		return p.failure(fmt.Errorf("provider returned no results"))
	}
	return response.Results
}

func (p Provider) failure(err error) map[string]metrics.Result {
	return map[string]metrics.Result{p.name + "/status": {Error: err.Error()}}
}
