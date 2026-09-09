package subprocess

import (
	"os"
	"path/filepath"
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
