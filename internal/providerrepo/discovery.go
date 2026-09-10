package providerrepo

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Discovery describes the latest metadata attempt for one configured
// repository. Repository metadata is retained when a later fetch fails, allowing callers
// to keep displaying known provider metadata during an outage.
type Discovery struct {
	URL        string      `json:"url"`
	Enabled    bool        `json:"enabled"`
	Available  bool        `json:"available"`
	Error      string      `json:"error,omitempty"`
	FetchedAt  time.Time   `json:"fetchedAt,omitempty"`
	Repository *Repository `json:"repository,omitempty"`
	Providers  []Provider  `json:"providers,omitempty"`
}

// Discover fetches metadata from enabled repositories. It never downloads an
// artifact and includes disabled repositories in the result for management UI.
// Previous results are used only as a metadata cache; they are not executed or
// treated as installed providers.
func Discover(ctx context.Context, settings Settings, client Client, previous []Discovery) []Discovery {
	previousByURL := make(map[string]Discovery, len(previous))
	for _, result := range previous {
		previousByURL[result.URL] = result
	}
	results := make([]Discovery, 0, len(settings.Repositories))
	for _, configured := range settings.Repositories {
		result := Discovery{URL: configured.URL, Enabled: configured.Enabled}
		if previousResult, ok := previousByURL[configured.URL]; ok {
			result.Repository = previousResult.Repository
			result.Providers = append([]Provider(nil), previousResult.Providers...)
			result.FetchedAt = previousResult.FetchedAt
		}
		if !configured.Enabled {
			results = append(results, result)
			continue
		}
		index, err := client.FetchRepository(ctx, configured.URL)
		if err != nil {
			result.Error = err.Error()
			logRepositoryFailure(configured.URL, err)
		} else {
			result.Available = true
			result.FetchedAt = time.Now().UTC()
			repository := index.Repository
			result.Repository = &repository
			result.Providers = append([]Provider(nil), index.Providers...)
		}
		results = append(results, result)
	}
	return results
}

func DiscoverConfigured(ctx context.Context, settings Settings, httpClient *http.Client, previous []Discovery) []Discovery {
	return Discover(ctx, settings, NewClient(httpClient), previous)
}

func logRepositoryFailure(repositoryURL string, err error) {
	slog.Warn("provider repository metadata unavailable", "repository", repositoryURL, "error", err)
}

func (d Discovery) String() string {
	if d.Error != "" {
		return fmt.Sprintf("%s: %s", d.URL, d.Error)
	}
	if d.Repository == nil {
		return d.URL
	}
	return fmt.Sprintf("%s: %s", d.URL, d.Repository.Name)
}
