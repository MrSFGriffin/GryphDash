package providerrepo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DiscoverInstalled returns executable providers in the managed cache for the
// current platform. The cache layout is deliberately the same layout used by
// Manager.Install; no provider is started while inspecting it.
func DiscoverInstalled(root string) ([]InstalledProvider, error) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read managed provider cache: %w", err)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	providers := []InstalledProvider{}
	for _, repository := range entries {
		if !repository.IsDir() {
			continue
		}
		providerEntries, err := os.ReadDir(filepath.Join(root, repository.Name()))
		if err != nil {
			return nil, fmt.Errorf("read managed repository %q: %w", repository.Name(), err)
		}
		for _, provider := range providerEntries {
			if !provider.IsDir() {
				continue
			}
			versions, err := os.ReadDir(filepath.Join(root, repository.Name(), provider.Name()))
			if err != nil {
				return nil, fmt.Errorf("read managed provider %q: %w", provider.Name(), err)
			}
			for _, version := range versions {
				if !version.IsDir() {
					continue
				}
				path := filepath.Join(root, repository.Name(), provider.Name(), version.Name(), platform, "gryphdash-provider-"+provider.Name())
				if runtime.GOOS == "windows" {
					path += ".exe"
				}
				info, statErr := os.Stat(path)
				if errors.Is(statErr, os.ErrNotExist) {
					continue
				}
				if statErr != nil {
					return nil, fmt.Errorf("stat managed provider %q: %w", provider.Name(), statErr)
				}
				// Windows does not expose Unix executable permission bits. The
				// .exe suffix selected above is the executable contract there;
				// requiring 0111 would reject every successfully installed
				// managed Windows provider.
				if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
					return nil, fmt.Errorf("managed provider %q is not an executable file", provider.Name())
				}
				providers = append(providers, InstalledProvider{
					RepositoryID: repository.Name(), ProviderID: provider.Name(), Version: version.Name(),
					GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Path: path,
				})
			}
		}
	}
	return providers, nil
}

// ValidateInstalled rejects ambiguous managed providers. A provider name may
// have one installed source/version only; otherwise runtime selection would
// depend on directory ordering.
func ValidateInstalled(providers []InstalledProvider) error {
	seen := make(map[string]InstalledProvider, len(providers))
	for _, provider := range providers {
		if strings.TrimSpace(provider.ProviderID) == "" {
			return errors.New("managed provider has an empty ID")
		}
		if previous, ok := seen[provider.ProviderID]; ok {
			if previous.RepositoryID != provider.RepositoryID || previous.Version != provider.Version {
				return fmt.Errorf("conflicting managed providers %q (%s/%s and %s/%s)", provider.ProviderID, previous.RepositoryID, previous.Version, provider.RepositoryID, provider.Version)
			}
			return fmt.Errorf("duplicate managed provider %q", provider.ProviderID)
		}
		seen[provider.ProviderID] = provider
	}
	return nil
}
