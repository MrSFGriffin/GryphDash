package providerrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const defaultMaxArtifactBytes int64 = 512 << 20

// Manager owns the managed provider cache and explicit lifecycle operations.
// It does not discover, install, or execute providers implicitly.
type Manager struct {
	Root             string
	HTTPClient       *http.Client
	MaxArtifactBytes int64
}

type InstalledProvider struct {
	RepositoryURL string `json:"repositoryUrl"`
	RepositoryID  string `json:"repositoryId"`
	ProviderID    string `json:"providerId"`
	Version       string `json:"version"`
	GOOS          string `json:"goos"`
	GOARCH        string `json:"goarch"`
	Path          string `json:"path"`
}

func NewManager(root string, httpClient *http.Client) Manager {
	return Manager{Root: root, HTTPClient: httpClient, MaxArtifactBytes: defaultMaxArtifactBytes}
}

func DefaultCacheDir(appName string) (string, error) {
	directory, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, appName, "providers"), nil
}

// Install downloads and verifies the selected platform artifact. A failed
// install leaves the existing verified binary untouched.
func (m Manager) Install(ctx context.Context, repositoryURL string, manifest Manifest, goos, goarch string) (InstalledProvider, error) {
	if err := ValidateHTTPSURL(repositoryURL); err != nil {
		return InstalledProvider{}, fmt.Errorf("repository: %w", err)
	}
	if err := Validate(manifest); err != nil {
		return InstalledProvider{}, fmt.Errorf("manifest: %w", err)
	}
	artifact, err := manifest.ArtifactFor(goos, goarch)
	if err != nil {
		return InstalledProvider{}, err
	}
	directory, path, err := m.artifactPath(repositoryURL, manifest, goos, goarch)
	if err != nil {
		return InstalledProvider{}, err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return InstalledProvider{}, fmt.Errorf("create provider cache: %w", err)
	}
	if err := removeTemporaryFiles(directory); err != nil {
		return InstalledProvider{}, fmt.Errorf("recover provider cache: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".install-*")
	if err != nil {
		return InstalledProvider{}, fmt.Errorf("create provider temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := m.download(ctx, temporary, artifact); err != nil {
		temporary.Close()
		return InstalledProvider{}, err
	}
	if err := temporary.Chmod(0700); err != nil {
		temporary.Close()
		return InstalledProvider{}, fmt.Errorf("set provider executable permissions: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return InstalledProvider{}, fmt.Errorf("sync provider temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return InstalledProvider{}, fmt.Errorf("close provider temporary file: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return InstalledProvider{}, fmt.Errorf("atomically install provider: %w", err)
	}
	return InstalledProvider{RepositoryURL: repositoryURL, RepositoryID: manifest.Repository.ID, ProviderID: manifest.Provider.ID, Version: manifest.Provider.Version, GOOS: goos, GOARCH: goarch, Path: path}, nil
}

// Update installs the new version first, then removes older versions for the
// same repository/provider/platform. If installation fails, the old version
// remains available.
func (m Manager) Update(ctx context.Context, repositoryURL string, manifest Manifest, goos, goarch string) (InstalledProvider, error) {
	installed, err := m.Install(ctx, repositoryURL, manifest, goos, goarch)
	if err != nil {
		return InstalledProvider{}, err
	}
	if err := m.removeOtherVersions(repositoryURL, manifest, goos, goarch, manifest.Provider.Version); err != nil {
		return InstalledProvider{}, err
	}
	return installed, nil
}

func (m Manager) Remove(repositoryURL, repositoryID, providerID, version, goos, goarch string) error {
	_, path, err := m.pathFor(repositoryURL, repositoryID, providerID, version, goos, goarch)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("remove provider: %w", err)
	}
	return removeEmptyParents(filepath.Dir(path), m.Root)
}

func (m Manager) InstalledPath(repositoryURL string, manifest Manifest, goos, goarch string) (string, error) {
	_, path, err := m.artifactPath(repositoryURL, manifest, goos, goarch)
	return path, err
}

func (m Manager) IsInstalled(repositoryURL string, manifest Manifest, goos, goarch string) (bool, error) {
	path, err := m.InstalledPath(repositoryURL, manifest, goos, goarch)
	if err != nil {
		return false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	checksum, err := checksum(file)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(checksum, manifest.Provider.Artifacts[goos+"-"+goarch].SHA256), nil
}

func (m Manager) download(ctx context.Context, destination *os.File, artifact Artifact) error {
	if err := ValidateHTTPSURL(artifact.URL); err != nil {
		return fmt.Errorf("artifact: %w", err)
	}
	client := m.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return fmt.Errorf("create artifact request: %w", err)
	}
	response, err := clientWithSecureRedirect(client).Do(request)
	if err != nil {
		return fmt.Errorf("download provider: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download provider: HTTP %s", response.Status)
	}
	limit := m.MaxArtifactBytes
	if limit <= 0 {
		limit = defaultMaxArtifactBytes
	}
	hash := sha256.New()
	reader := io.TeeReader(io.LimitReader(response.Body, limit+1), hash)
	count, err := io.Copy(destination, reader)
	if err != nil {
		return fmt.Errorf("write provider download: %w", err)
	}
	if count > limit {
		return fmt.Errorf("provider download exceeds %d-byte limit", limit)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), artifact.SHA256) {
		return errors.New("provider download checksum mismatch")
	}
	return nil
}

func (m Manager) artifactPath(repositoryURL string, manifest Manifest, goos, goarch string) (string, string, error) {
	if m.Root == "" {
		return "", "", errors.New("provider cache root is required")
	}
	return m.pathFor(repositoryURL, manifest.Repository.ID, manifest.Provider.ID, manifest.Provider.Version, goos, goarch)
}

func (m Manager) pathFor(repositoryURL, repositoryID, providerID, version, goos, goarch string) (string, string, error) {
	if m.Root == "" {
		return "", "", errors.New("provider cache root is required")
	}
	if err := ValidateHTTPSURL(repositoryURL); err != nil {
		return "", "", fmt.Errorf("repository: %w", err)
	}
	for field, value := range map[string]string{"repository ID": repositoryID, "provider ID": providerID, "version": version, "GOOS": goos, "GOARCH": goarch} {
		if err := validatePathComponent(field, value); err != nil {
			return "", "", err
		}
	}
	if !supportedPlatform(goos + "-" + goarch) {
		return "", "", fmt.Errorf("unsupported provider platform %s-%s", goos, goarch)
	}
	repositoryKey := sha256.Sum256([]byte(repositoryURL))
	platform := goos + "-" + goarch
	filename := "gryphdash-provider-" + providerID
	if goos == "windows" {
		filename += ".exe"
	}
	directory := filepath.Join(m.Root, hex.EncodeToString(repositoryKey[:]), providerID, version, platform)
	return directory, filepath.Join(directory, filename), nil
}

func (m Manager) removeOtherVersions(repositoryURL string, manifest Manifest, goos, goarch, keep string) error {
	root := filepath.Join(m.Root, repositoryKey(repositoryURL), manifest.Provider.ID)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("list provider versions: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == keep {
			continue
		}
		platformPath := filepath.Join(root, entry.Name(), goos+"-"+goarch)
		if err := os.RemoveAll(platformPath); err != nil {
			return fmt.Errorf("remove previous provider version: %w", err)
		}
	}
	return nil
}

func repositoryKey(repositoryURL string) string {
	key := sha256.Sum256([]byte(repositoryURL))
	return hex.EncodeToString(key[:])
}

func validatePathComponent(field, value string) error {
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsRune(value, filepath.Separator) {
		return fmt.Errorf("%s is not a safe cache path component", field)
	}
	return nil
}

func removeTemporaryFiles(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".install-") {
			if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func removeEmptyParents(directory, stop string) error {
	for directory != stop && directory != filepath.Dir(directory) {
		if err := os.Remove(directory); err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTEMPTY) {
				return nil
			}
			return err
		}
		directory = filepath.Dir(directory)
	}
	return nil
}

func checksum(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func clientWithSecureRedirect(client *http.Client) *http.Client {
	copy := *client
	copy.CheckRedirect = secureRedirect(client.CheckRedirect)
	return &copy
}
