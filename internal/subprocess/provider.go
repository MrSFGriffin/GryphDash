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

	"gryphdash/internal/dashboard"
	"gryphdash/internal/metrics"
	"gryphdash/internal/providerprotocol"
	"gryphdash/internal/providerrepo"
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
// gryphdash-provider-<name> convention. Managed providers in the default cache
// are included and take precedence. When directory is empty, the directory
// containing the running executable is searched for local providers.
func Discover(directory string) ([]Provider, error) {
	managedDirectory, _ := providerrepo.DefaultCacheDir("gryphdash")
	return DiscoverWithManaged(directory, managedDirectory)
}

// DiscoverWithManaged selects installed managed providers first, then local
// subprocess providers for names not present in the managed cache.
func DiscoverWithManaged(directory, managedDirectory string) ([]Provider, error) {
	managed, err := providerrepo.DiscoverInstalled(managedDirectory)
	if err != nil {
		return nil, err
	}
	if err := providerrepo.ValidateInstalled(managed); err != nil {
		return nil, err
	}
	selected := make(map[string]Provider, len(managed))
	for _, installed := range managed {
		selected[installed.ProviderID] = New(installed.ProviderID, installed.Path)
	}
	local, err := discoverLocal(directory)
	if err != nil {
		return nil, err
	}
	for _, provider := range local {
		if _, managed := selected[provider.Name()]; !managed {
			selected[provider.Name()] = provider
		}
	}
	providers := make([]Provider, 0, len(selected))
	for _, provider := range selected {
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name() < providers[j].Name() })
	return providers, nil
}

func discoverLocal(directory string) ([]Provider, error) {
	if directory != "" {
		return discoverDirectory(directory)
	}
	directories := make([]string, 0, 2)
	if executable, err := os.Executable(); err == nil {
		directories = append(directories, filepath.Dir(executable))
	}
	if working, err := os.Getwd(); err == nil {
		directories = append(directories, filepath.Join(working, "bin"))
	}
	seenDirectories := map[string]bool{}
	providers := []Provider{}
	seenNames := map[string]bool{}
	for _, candidate := range directories {
		if seenDirectories[candidate] {
			continue
		}
		seenDirectories[candidate] = true
		found, err := discoverDirectory(candidate)
		if err != nil {
			continue
		}
		for _, provider := range found {
			if !seenNames[provider.Name()] {
				seenNames[provider.Name()] = true
				providers = append(providers, provider)
			}
		}
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name() < providers[j].Name() })
	return providers, nil
}

func discoverDirectory(directory string) ([]Provider, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	providers := make([]Provider, 0, len(entries))
	seen := make(map[string]bool, len(entries))
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
		// A development bin directory can contain both an un-suffixed local
		// build and a platform-specific release build. Keep the first match so
		// they cannot produce duplicate provider catalogs.
		if seen[name] {
			continue
		}
		seen[name] = true
		providers = append(providers, New(name, filepath.Join(directory, entry.Name())))
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name() < providers[j].Name() })
	return providers, nil
}

func (p Provider) Name() string { return p.name }

func (p Provider) Read(parent context.Context) map[string]metrics.Result {
	response, err := p.run(parent, "read")
	if err != nil {
		return p.failure(err)
	}
	if response.Error != "" {
		return p.failure(fmt.Errorf("%s", response.Error))
	}
	if response.Results == nil {
		return p.failure(fmt.Errorf("provider returned no results"))
	}
	return response.Results
}

func (p Provider) Widgets(parent context.Context) (dashboard.WidgetCatalog, error) {
	response, err := p.run(parent, "widgets")
	if err != nil {
		return dashboard.WidgetCatalog{}, err
	}
	if response.Error != "" {
		return dashboard.WidgetCatalog{}, fmt.Errorf("%s", response.Error)
	}
	if len(response.Widgets.Widgets) == 0 {
		return dashboard.WidgetCatalog{}, fmt.Errorf("provider returned no widgets")
	}
	if err := dashboard.ValidateCatalog(response.Widgets); err != nil {
		return dashboard.WidgetCatalog{}, err
	}
	return response.Widgets, nil
}

func Catalog(parent context.Context, providers []Provider) dashboard.WidgetCatalog {
	catalog, err := CatalogWithError(parent, providers)
	if err != nil {
		slog.Warn("provider catalog rejected", "error", err)
	}
	return catalog
}

// CatalogWithError loads and merges provider catalogs, rejecting invalid
// catalogs and duplicate widget IDs across providers.
func CatalogWithError(parent context.Context, providers []Provider) (dashboard.WidgetCatalog, error) {
	merged := dashboard.WidgetCatalog{}
	for _, provider := range providers {
		catalog, err := provider.Widgets(parent)
		if err != nil {
			slog.Warn("provider widgets unavailable", "provider", provider.Name(), "error", err)
			continue
		}
		candidate, err := dashboard.MergeCatalog(merged, catalog)
		if err != nil {
			return merged, fmt.Errorf("provider %q: %w", provider.Name(), err)
		}
		merged = candidate
	}
	return merged, nil
}

func (p Provider) run(parent context.Context, method string) (providerprotocol.Response, error) {
	ctx, cancel := context.WithTimeout(parent, p.timeout)
	defer cancel()

	args := append([]string(nil), p.args...)
	if method == "widgets" {
		args = append(args, "-widgets")
	}
	cmd := exec.CommandContext(ctx, p.executable, args...)
	configureCommand(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	input, err := cmd.StdinPipe()
	if err != nil {
		return providerprotocol.Response{}, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return providerprotocol.Response{}, err
	}
	if err := cmd.Start(); err != nil {
		return providerprotocol.Response{}, err
	}

	if method == "widgets" {
		_ = input.Close()
	} else {
		requestErr := json.NewEncoder(input).Encode(providerprotocol.Request{Version: providerprotocol.Version, Method: method})
		_ = input.Close()
		if requestErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return providerprotocol.Response{}, requestErr
		}
	}

	var response providerprotocol.Response
	decodeErr := json.NewDecoder(bufio.NewReader(output)).Decode(&response)
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return providerprotocol.Response{}, ctx.Err()
	}
	if decodeErr != nil {
		return providerprotocol.Response{}, fmt.Errorf("reading provider response: %w", decodeErr)
	}
	if waitErr != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return providerprotocol.Response{}, fmt.Errorf("provider exited: %w: %s", waitErr, detail)
		}
		return providerprotocol.Response{}, fmt.Errorf("provider exited: %w", waitErr)
	}
	if response.Version != providerprotocol.Version {
		return providerprotocol.Response{}, fmt.Errorf("unsupported provider protocol version %d", response.Version)
	}
	if response.Provider != "" && response.Provider != p.name {
		slog.Warn("provider identity mismatch", "expected", p.name, "reported", response.Provider)
	}
	return response, nil
}

func (p Provider) failure(err error) map[string]metrics.Result {
	return map[string]metrics.Result{p.name + "/status": {Error: err.Error()}}
}
