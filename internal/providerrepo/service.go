package providerrepo

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

// ProviderState is the UI-facing lifecycle state of a repository provider.
type ProviderState string

const (
	StateAvailable   ProviderState = "available"
	StateInstalled   ProviderState = "installed"
	StateUpdate      ProviderState = "update"
	StateInstalling  ProviderState = "installing"
	StateUnavailable ProviderState = "unavailable"
	StateStale       ProviderState = "stale"
	StateError       ProviderState = "error"
)

// ProviderStatus combines repository metadata with local installation state.
type ProviderStatus struct {
	RepositoryURL    string        `json:"repositoryUrl"`
	RepositoryID     string        `json:"repositoryId"`
	ProviderID       string        `json:"providerId"`
	Name             string        `json:"name"`
	Version          string        `json:"version"`
	InstalledVersion string        `json:"installedVersion,omitempty"`
	State            ProviderState `json:"state"`
	Error            string        `json:"error,omitempty"`
}

// Service is the shared lifecycle boundary used by all three interfaces.
// Repository configuration is explicit; installing a provider is the only
// operation that downloads and executes no code (execution happens later via
// normal subprocess discovery).
type Service struct {
	settingsPath  string
	manager       Manager
	mu            sync.RWMutex
	discoveries   []Discovery
	installing    map[string]bool
	installErrors map[string]string
	onInstall     func()
}

func NewService(settingsPath, cacheRoot string, discoveries []Discovery) *Service {
	return &Service{settingsPath: settingsPath, manager: NewManager(cacheRoot, nil), discoveries: append([]Discovery(nil), discoveries...), installing: map[string]bool{}, installErrors: map[string]string{}}
}

// SetInstallCallback registers a callback invoked after an install or update
// finishes successfully. It is used by application runtimes to reload the
// newly installed provider without restarting.
func (s *Service) SetInstallCallback(callback func()) {
	s.mu.Lock()
	s.onInstall = callback
	s.mu.Unlock()
}

func (s *Service) Repositories() []Discovery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Discovery(nil), s.discoveries...)
}

func (s *Service) SetDiscoveries(discoveries []Discovery) {
	s.mu.Lock()
	s.discoveries = append([]Discovery(nil), discoveries...)
	s.mu.Unlock()
}

func (s *Service) AddRepository(rawURL string) ([]Discovery, error) {
	settings, err := LoadSettings(s.settingsPath)
	if err != nil {
		return nil, err
	}
	settings, err = Add(settings, rawURL)
	if err != nil {
		return nil, err
	}
	if err := SaveSettings(s.settingsPath, settings); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.discoveries = append(s.discoveries, Discovery{URL: rawURL, Enabled: true})
	result := append([]Discovery(nil), s.discoveries...)
	s.mu.Unlock()
	return result, nil
}

func (s *Service) SetRepositoryEnabled(rawURL string, enabled bool) ([]Discovery, error) {
	settings, err := LoadSettings(s.settingsPath)
	if err != nil {
		return nil, err
	}
	settings, err = SetEnabled(settings, rawURL, enabled)
	if err != nil {
		return nil, err
	}
	if err := SaveSettings(s.settingsPath, settings); err != nil {
		return nil, err
	}
	s.mu.Lock()
	for i := range s.discoveries {
		if s.discoveries[i].URL == rawURL {
			s.discoveries[i].Enabled = enabled
		}
	}
	result := append([]Discovery(nil), s.discoveries...)
	s.mu.Unlock()
	return result, nil
}

func (s *Service) RemoveRepository(rawURL string) ([]Discovery, error) {
	settings, err := LoadSettings(s.settingsPath)
	if err != nil {
		return nil, err
	}
	filtered := make([]ConfiguredRepository, 0, len(settings.Repositories))
	found := false
	for _, repository := range settings.Repositories {
		if repository.URL == rawURL {
			found = true
			continue
		}
		filtered = append(filtered, repository)
	}
	if !found {
		return nil, fmt.Errorf("provider repository %q is not configured", rawURL)
	}
	settings.Repositories = filtered
	if err := ValidateSettings(settings); err != nil {
		return nil, err
	}
	if err := SaveSettings(s.settingsPath, settings); err != nil {
		return nil, err
	}
	s.mu.Lock()
	filteredDiscoveries := s.discoveries[:0]
	for _, discovery := range s.discoveries {
		if discovery.URL != rawURL {
			filteredDiscoveries = append(filteredDiscoveries, discovery)
		}
	}
	s.discoveries = filteredDiscoveries
	result := append([]Discovery(nil), s.discoveries...)
	s.mu.Unlock()
	return result, nil
}

func (s *Service) Statuses() []ProviderStatus {
	installed, _ := DiscoverInstalled(s.manager.Root)
	byProvider := make(map[string]InstalledProvider, len(installed))
	for _, item := range installed {
		byProvider[item.ProviderID] = item
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	statuses := []ProviderStatus{}
	for _, repository := range s.discoveries {
		if repository.Repository == nil {
			continue
		}
		for _, provider := range repository.Providers {
			status := ProviderStatus{RepositoryURL: repository.URL, RepositoryID: repository.Repository.ID, ProviderID: provider.ID, Name: provider.Name, Version: provider.Version, State: StateAvailable}
			if repository.Error != "" {
				status.Error = repository.Error
				status.State = StateStale
			}
			if item, ok := byProvider[provider.ID]; ok && strings.Contains(item.Path, string(os.PathSeparator)+repositoryKey(repository.URL)+string(os.PathSeparator)) {
				status.InstalledVersion = item.Version
				if item.Version == provider.Version {
					status.State = StateInstalled
				} else {
					status.State = StateUpdate
				}
			}
			if s.installing[provider.ID] {
				status.State = StateInstalling
			}
			if err := s.installErrors[repository.URL+"\x00"+provider.ID]; err != "" {
				status.Error = err
				status.State = StateError
			}
			statuses = append(statuses, status)
		}
	}
	return statuses
}

func (s *Service) lifecycle(ctx context.Context, repositoryURL, providerID string, update bool) error {
	s.mu.RLock()
	var manifest *Manifest
	for _, discovery := range s.discoveries {
		if discovery.URL != repositoryURL || discovery.Repository == nil {
			continue
		}
		for _, provider := range discovery.Providers {
			if provider.ID == providerID {
				manifest = &Manifest{Version: ManifestVersion, Repository: *discovery.Repository, Provider: provider}
				break
			}
		}
		if manifest != nil {
			break
		}
	}
	s.mu.RUnlock()
	if manifest == nil {
		return fmt.Errorf("provider %q is unavailable in repository %q", providerID, repositoryURL)
	}
	key := providerID
	s.mu.Lock()
	if s.installing[key] {
		s.mu.Unlock()
		return fmt.Errorf("provider %q is already installing", providerID)
	}
	s.installing[key] = true
	s.mu.Unlock()
	installContext := context.WithoutCancel(ctx)
	go func() {
		var err error
		if update {
			_, err = s.manager.Update(installContext, repositoryURL, *manifest, runtime.GOOS, runtime.GOARCH)
		} else {
			_, err = s.manager.Install(installContext, repositoryURL, *manifest, runtime.GOOS, runtime.GOARCH)
		}
		s.mu.Lock()
		delete(s.installing, key)
		if err == nil {
			delete(s.installErrors, repositoryURL+"\x00"+providerID)
		} else {
			s.installErrors[repositoryURL+"\x00"+providerID] = err.Error()
		}
		for i := range s.discoveries {
			if s.discoveries[i].URL == repositoryURL && err != nil {
				s.discoveries[i].Error = err.Error()
			}
		}
		callback := s.onInstall
		s.mu.Unlock()
		if err == nil && callback != nil {
			callback()
		}
	}()
	return nil
}

func (s *Service) Install(ctx context.Context, repositoryURL, providerID string) error {
	return s.lifecycle(ctx, repositoryURL, providerID, false)
}
func (s *Service) Update(ctx context.Context, repositoryURL, providerID string) error {
	return s.lifecycle(ctx, repositoryURL, providerID, true)
}

func (s *Service) RemoveProvider(repositoryURL, providerID, version string) error {
	s.mu.RLock()
	var repositoryID string
	for _, discovery := range s.discoveries {
		if discovery.URL != repositoryURL || discovery.Repository == nil {
			continue
		}
		for _, provider := range discovery.Providers {
			if provider.ID == providerID {
				repositoryID = discovery.Repository.ID
				if version == "" {
					version = provider.Version
				}
				break
			}
		}
		if repositoryID != "" {
			break
		}
	}
	s.mu.RUnlock()
	if repositoryID == "" {
		return fmt.Errorf("provider %q is unavailable in repository %q", providerID, repositoryURL)
	}
	return s.manager.Remove(repositoryURL, repositoryID, providerID, version, runtime.GOOS, runtime.GOARCH)
}
