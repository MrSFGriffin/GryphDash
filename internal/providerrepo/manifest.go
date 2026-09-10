// Package providerrepo defines the versioned manifest exchanged by provider
// repositories. It deliberately contains no download or installation logic;
// reading a manifest is metadata-only.
package providerrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"runtime"
	"strings"

	"gryphdash/internal/dashboard"
)

const (
	// ManifestVersion is the currently supported repository manifest schema.
	ManifestVersion = 1
	ProtocolVersion = 1
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Manifest describes one provider release and its platform-specific binaries.
// It is embedded in a RepositoryIndex for repository discovery and retained as
// the installation input for a single selected provider.
type Manifest struct {
	Version    int        `json:"version"`
	Repository Repository `json:"repository"`
	Provider   Provider   `json:"provider"`
}

type Repository struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RepositoryIndex is the metadata-only entry point for one provider
// repository. Users configure this index, not individual provider manifests.
type RepositoryIndex struct {
	Version    int        `json:"version"`
	Repository Repository `json:"repository"`
	Providers  []Provider `json:"providers"`
}

type Provider struct {
	ID              string                  `json:"id"`
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Version         string                  `json:"version"`
	ProtocolVersion int                     `json:"protocolVersion"`
	Widgets         dashboard.WidgetCatalog `json:"widgets"`
	Artifacts       map[string]Artifact     `json:"artifacts"`
}

// Artifact identifies a release binary and its integrity checksum.
type Artifact struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Parse decodes and validates a manifest. Unknown fields are rejected so a
// future schema cannot be silently treated as a compatible one.
func Parse(data []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode provider manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return Manifest{}, errors.New("decode provider manifest: trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("decode provider manifest: trailing JSON: %w", err)
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate checks the manifest schema and every artifact URL and checksum.
func Validate(manifest Manifest) error {
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("unsupported provider manifest version %d", manifest.Version)
	}
	if err := validateIdentifier("repository ID", manifest.Repository.ID); err != nil {
		return err
	}
	if err := requireText("repository name", manifest.Repository.Name); err != nil {
		return err
	}
	if err := requireText("repository description", manifest.Repository.Description); err != nil {
		return err
	}
	provider := manifest.Provider
	if err := validateIdentifier("provider ID", provider.ID); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"provider name":        provider.Name,
		"provider description": provider.Description,
		"provider version":     provider.Version,
	} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if provider.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported provider protocol version %d", provider.ProtocolVersion)
	}
	if err := dashboard.ValidateCatalog(provider.Widgets); err != nil {
		return fmt.Errorf("provider widget catalog: %w", err)
	}
	if len(provider.Artifacts) == 0 {
		return errors.New("provider artifacts must not be empty")
	}
	for platform, artifact := range provider.Artifacts {
		if !supportedPlatform(platform) {
			return fmt.Errorf("unsupported provider platform %q", platform)
		}
		if err := validateHTTPSURL("artifact URL", artifact.URL); err != nil {
			return err
		}
		if !isSHA256(artifact.SHA256) {
			return fmt.Errorf("artifact %q has invalid SHA-256 checksum", platform)
		}
	}
	return nil
}

// ValidateRepositoryIndex rejects invalid provider groups and duplicate widget
// IDs before any provider artifact is downloaded or executed.
func ValidateRepositoryIndex(index RepositoryIndex) error {
	if index.Version != ManifestVersion {
		return fmt.Errorf("unsupported provider repository version %d", index.Version)
	}
	if err := validateRepository(index.Repository); err != nil {
		return err
	}
	if len(index.Providers) == 0 {
		return errors.New("provider repository must contain at least one provider")
	}
	providerIDs := make(map[string]bool, len(index.Providers))
	widgetIDs := map[string]bool{}
	for _, provider := range index.Providers {
		if err := Validate(Manifest{Version: index.Version, Repository: index.Repository, Provider: provider}); err != nil {
			return err
		}
		if providerIDs[provider.ID] {
			return fmt.Errorf("duplicate provider %q", provider.ID)
		}
		providerIDs[provider.ID] = true
		for _, widget := range provider.Widgets.Widgets {
			if widgetIDs[widget.ID] {
				return fmt.Errorf("duplicate widget ID %q", widget.ID)
			}
			widgetIDs[widget.ID] = true
		}
	}
	return nil
}

func validateRepository(repository Repository) error {
	if err := validateIdentifier("repository ID", repository.ID); err != nil {
		return err
	}
	if err := requireText("repository name", repository.Name); err != nil {
		return err
	}
	return requireText("repository description", repository.Description)
}

// ArtifactFor returns the artifact for GOOS/GOARCH, or an error when the
// manifest does not publish a binary for the current platform.
func (m Manifest) ArtifactFor(goos, goarch string) (Artifact, error) {
	key := goos + "-" + goarch
	artifact, ok := m.Provider.Artifacts[key]
	if !ok {
		return Artifact{}, fmt.Errorf("provider %q has no artifact for %s", m.Provider.ID, key)
	}
	return artifact, nil
}

// CurrentArtifact selects the artifact for the running process.
func (m Manifest) CurrentArtifact() (Artifact, error) {
	return m.ArtifactFor(runtime.GOOS, runtime.GOARCH)
}

func validateIdentifier(field, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s must contain only lowercase letters, digits, '.', '_' or '-'", field)
	}
	return nil
}

func requireText(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return nil
}

func validateHTTPSURL(field, raw string) error {
	if err := ValidateHTTPSURL(raw); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	return nil
}

// ValidateHTTPSURL rejects insecure or malformed repository and artifact
// endpoints. SHA-256 does not authenticate the publisher, so repository URLs
// remain an explicit user-trust boundary.
func ValidateHTTPSURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("must be an HTTPS URL")
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func supportedPlatform(platform string) bool {
	switch platform {
	case "linux-amd64", "windows-amd64", "darwin-amd64", "darwin-arm64":
		return true
	default:
		return false
	}
}
