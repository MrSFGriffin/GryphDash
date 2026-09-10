package providerrepo

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverInstalledFindsCurrentPlatformExecutables(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "repo-hash", "currency", "1.2.3", runtime.GOOS+"-"+runtime.GOARCH, "gryphdash-provider-currency")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("provider"), 0700); err != nil {
		t.Fatal(err)
	}
	providers, err := DiscoverInstalled(root)
	if err != nil || len(providers) != 1 {
		t.Fatalf("providers = %+v, error = %v", providers, err)
	}
	if providers[0].ProviderID != "currency" || providers[0].Version != "1.2.3" || providers[0].Path != path {
		t.Fatalf("provider = %+v", providers[0])
	}
}

func TestValidateInstalledRejectsConflictingSourcesAndVersions(t *testing.T) {
	for name, providers := range map[string][]InstalledProvider{
		"sources":  {{ProviderID: "currency", RepositoryID: "one", Version: "1.0.0"}, {ProviderID: "currency", RepositoryID: "two", Version: "1.0.0"}},
		"versions": {{ProviderID: "currency", RepositoryID: "one", Version: "1.0.0"}, {ProviderID: "currency", RepositoryID: "one", Version: "2.0.0"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateInstalled(providers); err == nil || !strings.Contains(err.Error(), "conflicting") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
