package providerrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repositories.json")
	settings, err := Add(DefaultSettings(), "https://example.test/providers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repositories) != 2 || got.Repositories[1].URL != "https://example.test/providers.json" || !got.Repositories[1].Enabled {
		t.Fatalf("loaded settings = %+v", got)
	}
	mode, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode.Mode().Perm() != 0600 {
		t.Fatalf("settings mode = %o, want 600", mode.Mode().Perm())
	}
}

func TestRepositorySettingsMissingUsesCoreDefault(t *testing.T) {
	got, err := LoadSettings(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repositories) != 1 || got.Repositories[0].URL != DefaultCoreRepositoryURL || !got.Repositories[0].Enabled {
		t.Fatalf("default settings = %+v", got)
	}
}

func TestRepositorySettingsMigratesLegacyCoreManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repositories.json")
	data := []byte(`{"repositories":[{"url":"https://raw.githubusercontent.com/MrSFGriffin/GryphDash-Providers/main/manifest.json","enabled":true}]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repositories) != 1 || got.Repositories[0].URL != DefaultCoreRepositoryURL {
		t.Fatalf("migrated settings = %+v", got)
	}
	for _, repository := range got.Repositories {
		if repository.URL == legacyCoreRepositoryURL {
			t.Fatal("legacy repository URL was retained")
		}
		if !repository.Enabled {
			t.Fatalf("migrated repository disabled: %+v", repository)
		}
	}
}

func TestRepositorySettingsCollapsesLegacyProviderManifests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repositories.json")
	data := []byte(`{"repositories":[{"url":"https://raw.githubusercontent.com/MrSFGriffin/GryphDash-Providers/main/manifests/manifest-codex.json","enabled":true},{"url":"https://raw.githubusercontent.com/MrSFGriffin/GryphDash-Providers/main/manifests/manifest-currency.json","enabled":false},{"url":"https://raw.githubusercontent.com/MrSFGriffin/GryphDash-Providers/main/manifests/manifest-openrouter.json","enabled":true}]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repositories) != 1 || got.Repositories[0].URL != DefaultCoreRepositoryURL || !got.Repositories[0].Enabled {
		t.Fatalf("migrated settings = %+v", got)
	}
}

func TestRepositorySettingsRejectUnsafeAndDuplicateRepositories(t *testing.T) {
	if _, err := Add(DefaultSettings(), "http://example.test/providers.json"); err == nil {
		t.Fatal("insecure repository unexpectedly accepted")
	}
	if _, err := Add(DefaultSettings(), DefaultCoreRepositoryURL); err == nil || !strings.Contains(err.Error(), "already configured") {
		t.Fatalf("duplicate repository error = %v", err)
	}
	settings := Settings{Repositories: []ConfiguredRepository{
		{URL: "https://example.test/providers.json", Enabled: true},
		{URL: "https://example.test/providers.json", Enabled: false},
	}}
	if err := ValidateSettings(settings); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate validation error = %v", err)
	}
}

func TestSetEnabled(t *testing.T) {
	settings, err := SetEnabled(DefaultSettings(), DefaultCoreRepositoryURL, false)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Repositories[0].Enabled {
		t.Fatal("repository remained enabled")
	}
	if _, err := SetEnabled(settings, "https://example.test/missing.json", true); err == nil {
		t.Fatal("missing repository unexpectedly updated")
	}
}
