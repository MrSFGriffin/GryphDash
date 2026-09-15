package subprocess

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gryphdash/internal/dashboard"
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

func TestDescribeUsesProtocolV2AndDescribeFlag(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "provider.sh")
	contents := `#!/bin/sh
if [ "$1" != "-describe" ]; then exit 11; fi
read request
case "$request" in *'"version":2'*'"method":"describe"'*) ;; *) exit 12 ;; esac
printf '%s\n' '{"version":2,"provider":"fixture","description":{"id":"fixture","name":"Fixture","description":"Test provider","protocolVersion":2,"sources":[{"id":"fixture/value","name":"Value","description":"A value","schema":{}}]}}'
`
	if err := os.WriteFile(script, []byte(contents), 0700); err != nil {
		t.Fatal(err)
	}
	description, err := New("fixture", script).Describe(context.Background())
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if description.ID != "fixture" || len(description.Sources) != 1 {
		t.Fatalf("description = %+v", description)
	}
}

func TestDescribeRejectsProtocolV1(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "provider.sh")
	contents := `#!/bin/sh
printf '%s\n' '{"version":1,"provider":"fixture","description":{"id":"fixture","name":"Fixture","description":"Test provider","protocolVersion":1,"sources":[]}}'
`
	if err := os.WriteFile(script, []byte(contents), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := New("fixture", script).Describe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported provider protocol version") {
		t.Fatalf("Describe() error = %v", err)
	}
}

func TestDescriptionsRejectDuplicateSourceIDs(t *testing.T) {
	directory := t.TempDir()
	makeProvider := func(id string) string {
		script := filepath.Join(directory, id+".sh")
		description := dashboard.ProviderDescription{ID: id, Name: id, Description: "Test provider", ProtocolVersion: 2, Sources: []dashboard.SourceDescription{{ID: "shared/value", Name: "Value", Description: "A value", Schema: map[string]any{}}}}
		response, _ := json.Marshal(map[string]any{"version": 2, "provider": id, "description": description})
		contents := "#!/bin/sh\nprintf '%s\\n' '" + string(response) + "'\n"
		if err := os.WriteFile(script, []byte(contents), 0700); err != nil {
			t.Fatal(err)
		}
		return script
	}
	providers := []Provider{New("alpha", makeProvider("alpha")), New("beta", makeProvider("beta"))}
	_, err := DescriptionsWithError(context.Background(), providers)
	if err == nil || !strings.Contains(err.Error(), "duplicate source ID") {
		t.Fatalf("DescriptionsWithError() error = %v", err)
	}
}
