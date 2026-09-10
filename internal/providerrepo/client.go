package providerrepo

import (
	"context"
	"crypto/tls"
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
	if err := ValidateHTTPSURL(endpoint); err != nil {
		return Manifest{}, fmt.Errorf("manifest endpoint: %w", err)
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
		return Manifest{}, fmt.Errorf("create manifest request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return Manifest{}, fmt.Errorf("fetch manifest: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Manifest{}, fmt.Errorf("fetch manifest: HTTP %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return Manifest{}, fmt.Errorf("manifest exceeds %d-byte limit", maxBytes)
	}
	manifest, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return manifest, nil
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
