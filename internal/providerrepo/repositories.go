package providerrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// DefaultCoreRepositoryURL is the built-in source for the separately released
// core providers. It can be disabled in local configuration, but remains the
// default when no repository configuration has been created yet.
const DefaultCoreRepositoryURL = "https://raw.githubusercontent.com/MrSFGriffin/GryphDash-Providers/main/manifest.json"

type ConfiguredRepository struct {
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type Settings struct {
	Repositories []ConfiguredRepository `json:"repositories"`
}

func DefaultSettings() Settings {
	return Settings{Repositories: []ConfiguredRepository{{URL: DefaultCoreRepositoryURL, Enabled: true}}}
}

func SettingsPath(appName string) (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, appName, "repositories.json"), nil
}

func LoadSettings(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}
	if err := ValidateSettings(settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

// SaveSettings atomically replaces the settings file and restricts it to the
// current user because repository configuration controls native executables.
func SaveSettings(path string, settings Settings) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".repositories-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func ValidateSettings(settings Settings) error {
	if len(settings.Repositories) == 0 {
		return errors.New("at least one provider repository is required")
	}
	seen := make(map[string]bool, len(settings.Repositories))
	for _, repository := range settings.Repositories {
		if err := ValidateHTTPSURL(repository.URL); err != nil {
			return fmt.Errorf("repository %q: %w", repository.URL, err)
		}
		parsed, _ := url.Parse(repository.URL)
		canonical := parsed.String()
		if seen[canonical] {
			return fmt.Errorf("duplicate provider repository %q", repository.URL)
		}
		seen[canonical] = true
	}
	return nil
}

func Add(settings Settings, rawURL string) (Settings, error) {
	if err := ValidateHTTPSURL(rawURL); err != nil {
		return Settings{}, fmt.Errorf("repository %q: %w", rawURL, err)
	}
	parsed, _ := url.Parse(rawURL)
	for _, repository := range settings.Repositories {
		if repository.URL == parsed.String() {
			return Settings{}, fmt.Errorf("repository %q is already configured", rawURL)
		}
	}
	settings.Repositories = append(settings.Repositories, ConfiguredRepository{URL: parsed.String(), Enabled: true})
	if err := ValidateSettings(settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func SetEnabled(settings Settings, rawURL string, enabled bool) (Settings, error) {
	for i := range settings.Repositories {
		if settings.Repositories[i].URL == rawURL {
			settings.Repositories[i].Enabled = enabled
			return settings, ValidateSettings(settings)
		}
	}
	return Settings{}, fmt.Errorf("provider repository %q is not configured", rawURL)
}
