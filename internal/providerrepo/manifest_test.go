package providerrepo

import (
	"encoding/json"
	"strings"
	"testing"

	"gryphdash/internal/dashboard"
)

func TestParseManifest(t *testing.T) {
	manifest := validManifest()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Provider.ID != "currency" {
		t.Fatalf("provider ID = %q", parsed.Provider.ID)
	}
	artifact, err := parsed.ArtifactFor("linux", "amd64")
	if err != nil || artifact.URL != "https://example.test/currency-linux-amd64" {
		t.Fatalf("linux artifact = %+v, error = %v", artifact, err)
	}
}

func TestParseManifestRejectsInvalidSecurityAndSchemaFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{"http artifact", func(m *Manifest) {
			m.Provider.Artifacts["linux-amd64"] = Artifact{URL: "http://example.test/bin", SHA256: strings.Repeat("a", 64)}
		}, "HTTPS"},
		{"bad checksum", func(m *Manifest) {
			m.Provider.Artifacts["linux-amd64"] = Artifact{URL: "https://example.test/bin", SHA256: "not-a-checksum"}
		}, "SHA-256"},
		{"unsupported platform", func(m *Manifest) { m.Provider.Artifacts["plan9-amd64"] = m.Provider.Artifacts["linux-amd64"] }, "platform"},
		{"duplicate widget ID", func(m *Manifest) {
			m.Provider.Widgets.Widgets = append(m.Provider.Widgets.Widgets, m.Provider.Widgets.Widgets[0])
		}, "duplicate widget ID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(func() Manifest { m := validManifest(); test.mutate(&m); return m }())
			if err != nil {
				t.Fatal(err)
			}
			_, err = Parse(data)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseManifestRejectsUnknownFields(t *testing.T) {
	data := []byte(`{"version":1,"repository":{"id":"core","name":"Core","description":"Core providers"},"provider":{"id":"currency","name":"Currency","description":"Rates","version":"1.0.0","protocolVersion":1,"widgets":{"widgets":[]},"artifacts":{},"unexpected":true}}`)
	_, err := Parse(data)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Parse() error = %v", err)
	}
}

func TestArtifactForMissingPlatform(t *testing.T) {
	_, err := validManifest().ArtifactFor("windows", "arm64")
	if err == nil || !strings.Contains(err.Error(), "no artifact") {
		t.Fatalf("ArtifactFor() error = %v", err)
	}
}

func TestValidateHTTPSURL(t *testing.T) {
	for _, raw := range []string{"https://example.test/manifest.json", "https://example.test"} {
		if err := ValidateHTTPSURL(raw); err != nil {
			t.Errorf("ValidateHTTPSURL(%q) error = %v", raw, err)
		}
	}
	for _, raw := range []string{"http://example.test/manifest.json", "https://", "https://user@example.test/manifest.json"} {
		if err := ValidateHTTPSURL(raw); err == nil {
			t.Errorf("ValidateHTTPSURL(%q) unexpectedly succeeded", raw)
		}
	}
}

func validManifest() Manifest {
	return Manifest{
		Version:    ManifestVersion,
		Repository: Repository{ID: "core", Name: "Core providers", Description: "GryphDash providers"},
		Provider: Provider{
			ID: "currency", Name: "Currency", Description: "Exchange rates", Version: "1.0.0", ProtocolVersion: ProtocolVersion,
			Widgets: dashboard.WidgetCatalog{Widgets: []dashboard.WidgetConfig{{ID: "currency/rate", Group: "Currency", Name: "Rate", Description: "A rate", Width: 4, Height: 2, Logic: dashboard.WidgetLogic{Type: "scalar"}}}},
			Artifacts: map[string]Artifact{
				"linux-amd64":   {URL: "https://example.test/currency-linux-amd64", SHA256: strings.Repeat("a", 64)},
				"windows-amd64": {URL: "https://example.test/currency-windows-amd64.exe", SHA256: strings.Repeat("b", 64)},
			},
		},
	}
}
