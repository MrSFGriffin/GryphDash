package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	dashboardpkg "gryphdash/internal/dashboard"
)

const widgetParityDirectory = "testdata/widget-parity"

type widgetParityResult struct {
	Data    map[string]any `json:"data"`
	Updated string         `json:"updated"`
	Error   string         `json:"error,omitempty"`
}

type widgetParitySnapshot struct {
	Results map[string]widgetParityResult `json:"results"`
}

func loadWidgetParityCatalog(t *testing.T) widgetCatalog {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(widgetParityDirectory, "current-provider-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := dashboardpkg.ParseCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func loadWidgetParitySnapshot(t *testing.T) snapshot {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(widgetParityDirectory, "current-provider-snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture widgetParitySnapshot
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	results := make(map[string]result, len(fixture.Results))
	for source, item := range fixture.Results {
		updated, err := time.Parse(time.RFC3339, item.Updated)
		if err != nil {
			t.Fatalf("%s updated: %v", source, err)
		}
		results[source] = result{Data: item.Data, Updated: updated, Error: item.Error}
	}
	return snapshot{Results: results}
}

func currentWidgetParityDashboard(t *testing.T) dashboard {
	t.Helper()
	return buildDashboardWithCatalog(loadWidgetParitySnapshot(t), loadWidgetParityCatalog(t))
}

func loadWidgetParityJSON(t *testing.T, name string, target any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(widgetParityDirectory, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func TestCurrentWidgetParity(t *testing.T) {
	got := currentWidgetParityDashboard(t)
	var want dashboard
	loadWidgetParityJSON(t, "current-dashboard.json", &want)
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("current widget parity mismatch\\nwant:\\n%s\\n\\ngot:\\n%s", wantJSON, gotJSON)
	}
}

func TestCurrentDefaultWidgetSelection(t *testing.T) {
	dashboard := currentWidgetParityDashboard(t)
	got := make([]string, 0)
	for _, widget := range dashboard.Widgets {
		if widget.Default {
			got = append(got, widget.ID)
		}
	}
	var want []string
	loadWidgetParityJSON(t, "current-default-selection.json", &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default widgets = %#v, want %#v", got, want)
	}
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func plainTUICard(w widget) string {
	return ansiEscape.ReplaceAllString(renderTUICard(w), "")
}

func TestCurrentTUICardParity(t *testing.T) {
	dashboard := currentWidgetParityDashboard(t)
	got := make(map[string]string, len(dashboard.Widgets))
	for _, widget := range dashboard.Widgets {
		got[widget.ID] = plainTUICard(widget)
	}
	var want map[string]string
	loadWidgetParityJSON(t, "current-tui-cards.json", &want)
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		wantJSON, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("current TUI card parity mismatch\\nwant:\\n%s\\n\\ngot:\\n%s", wantJSON, gotJSON)
	}
}

func TestCurrentWidgetParityFixtureCoverage(t *testing.T) {
	dashboard := currentWidgetParityDashboard(t)
	groups := map[string]int{}
	for _, widget := range dashboard.Widgets {
		groups[widget.Group]++
	}
	if groups["Codex"] != 36 || groups["OpenRouter"] != 9 || groups["Currency"] != 4 {
		t.Fatalf("widget group coverage = %#v", groups)
	}
	for _, id := range []string{
		"codex/bucket/codex/primary",
		"codex/bucket/codex/secondary",
		"codex/resets/details",
		"codex/usage/daily",
		"openrouter/expires",
		"currency/eur-usd",
	} {
		found := false
		for _, widget := range dashboard.Widgets {
			if widget.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("fixture omitted %s", id)
		}
	}
}
