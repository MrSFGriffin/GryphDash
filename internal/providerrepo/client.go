package providerrepo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const defaultMaxManifestBytes int64 = 4 << 20

// Client fetches repository metadata only. It never requests an artifact URL
// and does not execute anything returned by a manifest.
type Client struct {
	HTTPClient      *http.Client
	MaxManifestSize int64
}

func NewClient(httpClient *http.Client) Client {
	return Client{HTTPClient: httpClient, MaxManifestSize: defaultMaxManifestBytes}
}

func (c Client) FetchManifest(ctx context.Context, endpoint string) (Manifest, error) {
	data, err := c.fetch(ctx, endpoint, "manifest")
	if err != nil {
		return Manifest{}, err
	}
	manifest, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
}

// FetchRepository obtains a repository index only. It does not request a
// provider artifact or execute provider code.
func (c Client) FetchRepository(ctx context.Context, endpoint string) (RepositoryIndex, error) {
	data, err := c.fetch(ctx, endpoint, "repository")
	if err != nil {
		return RepositoryIndex{}, err
	}
	var index RepositoryIndex
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&index); err != nil {
		return RepositoryIndex{}, fmt.Errorf("parse repository: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return RepositoryIndex{}, fmt.Errorf("parse repository: trailing JSON")
	} else if !errors.Is(err, io.EOF) {
		return RepositoryIndex{}, fmt.Errorf("parse repository: trailing JSON: %w", err)
	}
	if err := ValidateRepositoryIndex(index); err != nil {
		return RepositoryIndex{}, fmt.Errorf("parse repository: %w", err)
	}
	return index, nil
}

func (c Client) fetch(ctx context.Context, endpoint, kind string) ([]byte, error) {
	if err := ValidateHTTPSURL(endpoint); err != nil {
		return nil, fmt.Errorf("%s endpoint: %w", kind, err)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	client := *httpClient
	client.CheckRedirect = secureRedirect(httpClient.CheckRedirect)
	maxBytes := c.MaxManifestSize
	if maxBytes <= 0 {
		maxBytes = defaultMaxManifestBytes
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", kind, err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", kind, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch %s: HTTP %s", kind, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", kind, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s exceeds %d-byte limit", kind, maxBytes)
	}
	return data, nil
}

func secureRedirect(previous func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if err := ValidateHTTPSURL(request.URL.String()); err != nil {
			return fmt.Errorf("redirected manifest endpoint: %w", err)
		}
		if previous != nil {
			return previous(request, via)
		}
		return nil
	}
}

// TLSClient is useful for local HTTPS fixtures and callers that need to
// provide custom transport settings. Production callers should use normal
// certificate verification and should not set InsecureSkipVerify.
func TLSClient(config *tls.Config) *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: config}}
}
