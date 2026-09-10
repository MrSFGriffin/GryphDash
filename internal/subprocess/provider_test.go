package subprocess

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscover(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"gryphdash-provider-zeta", "gryphdash-provider-alpha", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "gryphdash-provider-directory"), 0700); err != nil {
		t.Fatal(err)
	}
	providers, err := Discover(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 || providers[0].Name() != "alpha" || providers[1].Name() != "zeta" {
		t.Fatalf("providers = %+v", providers)
	}
}

func TestDiscoverWithManagedPrefersManagedProvider(t *testing.T) {
	local, managed := t.TempDir(), t.TempDir()
	localPath := filepath.Join(local, "gryphdash-provider-currency")
	if err := os.WriteFile(localPath, nil, 0700); err != nil {
		t.Fatal(err)
	}
	managedPath := filepath.Join(managed, "repo-hash", "currency", "2.0.0", runtime.GOOS+"-"+runtime.GOARCH, "gryphdash-provider-currency")
	if runtime.GOOS == "windows" {
		managedPath += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(managedPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedPath, nil, 0700); err != nil {
		t.Fatal(err)
	}
	providers, err := DiscoverWithManaged(local, managed)
	if err != nil || len(providers) != 1 {
		t.Fatalf("providers = %+v, error = %v", providers, err)
	}
	if providers[0].Name() != "currency" || providers[0].executable != managedPath {
		t.Fatalf("selected provider = %+v", providers[0])
	}
}

func TestDiscoverWithManagedFallsBackToLocalProvider(t *testing.T) {
	local, managed := t.TempDir(), t.TempDir()
	localPath := filepath.Join(local, "gryphdash-provider-currency")
	if err := os.WriteFile(localPath, nil, 0700); err != nil {
		t.Fatal(err)
	}
	providers, err := DiscoverWithManaged(local, managed)
	if err != nil || len(providers) != 1 || providers[0].executable != localPath {
		t.Fatalf("providers = %+v, error = %v", providers, err)
	}
}

func TestDiscoverEmptyDirectory(t *testing.T) {
	providers, err := Discover("")
	if err != nil {
		t.Fatalf("providers = %v, err = %v", providers, err)
	}
}

func TestDiscoverNormalizesWindowsBuildName(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "gryphdash-provider-currency-windows-amd64.exe")
	if err := os.WriteFile(path, nil, 0700); err != nil {
		t.Fatal(err)
	}
	providers, err := Discover(directory)
	if err != nil || len(providers) != 1 || providers[0].Name() != "currency" {
		t.Fatalf("providers = %+v, err = %v", providers, err)
	}
}
