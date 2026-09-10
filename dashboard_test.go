package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	collectorpkg "gryphdash/internal/collector"
	dashboardpkg "gryphdash/internal/dashboard"
	"gryphdash/internal/providerrepo"
)

func testCatalog(t *testing.T) widgetCatalog {
	t.Helper()
	widgets := make([]dashboardpkg.WidgetConfig, 0, 30)
	for _, entry := range []struct {
		id, name, path, kind string
		defaultWidget        bool
	}{
		{"codex/bucket/{bucket}/primary", "Primary limit", "primary", "limitWindow", true},
		{"codex/bucket/{bucket}/secondary", "Secondary limit", "secondary", "limitWindow", true},
		{"codex/bucket/{bucket}/credits/balance", "Credits remaining", "credits.balance", "scalar", true},
		{"codex/bucket/{bucket}/credits/hasCredits", "Credits available", "credits.hasCredits", "scalar", false},
		{"codex/bucket/{bucket}/planType", "Bucket plan", "planType", "scalar", false},
	} {
		widgets = append(widgets, dashboardpkg.WidgetConfig{ID: entry.id, Group: "Codex", Name: entry.name, Description: "Test widget", Scope: "limitBuckets", Default: entry.defaultWidget, Width: 4, Height: 4, Logic: dashboardpkg.WidgetLogic{Type: entry.kind, Source: "codex/limits", Path: entry.path}})
	}
	for _, entry := range []struct {
		id, name, source, path, kind string
		defaultWidget                bool
	}{
		{"codex/resets/count", "Available resets", "codex/limits", "rateLimitResetCredits.availableCount", "scalar", true},
		{"codex/resets/details", "Earned reset details", "codex/limits", "rateLimitResetCredits", "resetDetails", false},
		{"codex/usage/lifetimeTokens", "Lifetime tokens", "codex/usage", "summary.lifetimeTokens", "scalar", true},
		{"codex/usage/currentStreakDays", "Current streak", "codex/usage", "summary.currentStreakDays", "scalar", true},
		{"codex/usage/longestStreakDays", "Longest streak", "codex/usage", "summary.longestStreakDays", "scalar", false},
		{"codex/usage/daily", "Daily token activity", "codex/usage", "dailyUsageBuckets", "daily", true},
		{"codex/account/plan", "Account plan", "codex/account", "account.planType", "scalar", true},
	} {
		widgets = append(widgets, dashboardpkg.WidgetConfig{ID: entry.id, Group: "Codex", Name: entry.name, Description: "Test widget", Default: entry.defaultWidget, Width: 4, Height: 4, Logic: dashboardpkg.WidgetLogic{Type: entry.kind, Source: entry.source, Path: entry.path}})
	}
	for i := 0; i < 9; i++ {
		widgets = append(widgets, dashboardpkg.WidgetConfig{ID: fmt.Sprintf("codex/test/%d", i), Group: "Codex", Name: fmt.Sprintf("Codex test %d", i), Description: "Test widget", Width: 4, Height: 4, Logic: dashboardpkg.WidgetLogic{Type: "scalar", Source: "codex/test", Path: fmt.Sprintf("value%d", i)}})
	}
	for i := 0; i < 9; i++ {
		widgets = append(widgets, dashboardpkg.WidgetConfig{ID: fmt.Sprintf("openrouter/test/%d", i), Group: "OpenRouter", Name: fmt.Sprintf("OpenRouter test %d", i), Description: "Test widget", Width: 4, Height: 4, Logic: dashboardpkg.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: fmt.Sprintf("value%d", i), URL: "https://openrouter.ai/api/v1/key", Method: "GET"}})
	}
	catalog, err := dashboardpkg.MergeCatalog(dashboardpkg.WidgetCatalog{}, dashboardpkg.WidgetCatalog{Widgets: widgets})
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func fixture(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

type testReader struct {
	name string
	read func(context.Context) map[string]result
}

func (r testReader) Name() string                               { return r.name }
func (r testReader) Read(ctx context.Context) map[string]result { return r.read(ctx) }

func TestProviderRepositoryMetadataAPI(t *testing.T) {
	c := collectorpkg.New(collectorpkg.Options{})
	repositories := []providerrepo.Discovery{
		{URL: "https://example.test/manifest.json", Available: false, Error: "repository unavailable"},
		{URL: "https://core.example/repository.json", Available: true, Repository: &providerrepo.Repository{ID: "core", Name: "Core", Description: "Test"}, Providers: []providerrepo.Provider{{ID: "currency", Name: "Currency", Description: "Test", Version: "1.0.0"}}},
	}
	w := httptest.NewRecorder()
	newHandlerWithCatalogAndRepositories(c, testCatalog(t), repositories).ServeHTTP(w, httptest.NewRequest("GET", "/api/provider-repositories", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var got []providerrepo.Discovery
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Error == "" || len(got[1].Providers) != 1 || got[1].Providers[0].ID != "currency" {
		t.Fatalf("metadata = %+v", got)
	}
}

func TestDashboardMetrics(t *testing.T) {
	now := time.Unix(1700000000, 0)
	limits := fixture(t, `{"buckets":{"codex":{"primary":{"usedPercent":25,"windowDurationMins":300,"resetsAt":1700003600},"secondary":{"usedPercent":4,"windowDurationMins":10080},"credits":{"balance":"0","hasCredits":false,"unlimited":false}},"other":{"primary":{"usedPercent":10,"windowDurationMins":60}}},"rateLimitResetCredits":{"availableCount":2,"credits":[]}}`)
	usage := fixture(t, `{"summary":{"lifetimeTokens":123,"currentStreakDays":0},"dailyUsageBuckets":[{"startDate":"2026-01-01","tokens":0},{"startDate":"2026-01-02","tokens":50}]}`)
	c := collectorpkg.New(collectorpkg.Options{})
	c.SetSnapshot(snapshot{Results: map[string]result{"codex/limits": {Data: limits, Updated: now}, "codex/usage": {Data: usage, Updated: now}}})
	w := httptest.NewRecorder()
	newHandlerWithCatalog(c, testCatalog(t)).ServeHTTP(w, httptest.NewRequest("GET", "/api/widgets", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var data dashboard
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	widgets := map[string]widget{}
	for _, widget := range data.Widgets {
		if _, exists := widgets[widget.ID]; exists {
			t.Fatalf("duplicate widget ID %q", widget.ID)
		}
		widgets[widget.ID] = widget
		if widget.Width < 1 || widget.Height < 2 {
			t.Fatalf("invalid default size: %+v", widget)
		}
	}
	for id, want := range map[string]string{
		"codex/bucket/codex/primary":            "75%",
		"codex/bucket/codex/secondary":          "96%",
		"codex/bucket/codex/credits/balance":    "0",
		"codex/bucket/codex/credits/hasCredits": "No",
		"codex/bucket/other/primary":            "90%",
		"codex/resets/count":                    "2",
		"codex/usage/lifetimeTokens":            "123",
		"codex/usage/currentStreakDays":         "0",
		"codex/usage/longestStreakDays":         "Unavailable",
	} {
		if got := widgets[id].Value; got != want {
			t.Errorf("%s: got %q, want %q", id, got, want)
		}
	}
	if widgets["codex/bucket/codex/primary"].Title != "5-hour limit" || widgets["codex/bucket/codex/secondary"].Title != "Weekly limit" {
		t.Fatal("window titles")
	}
	days := widgets["codex/usage/daily"].Days
	if len(days) != 2 || days[0].Percent != 100 || days[1].Percent != 0 {
		t.Fatalf("bad daily chart: %+v", days)
	}
	if !strings.Contains(w.Body.String(), "OpenRouter") {
		t.Fatal("configured OpenRouter widgets are missing")
	}
	if len(testCatalog(t).Widgets) < 20 {
		t.Fatalf("widget catalog unexpectedly small: %d", len(testCatalog(t).Widgets))
	}
	for _, c := range testCatalog(t).Widgets {
		if c.Group != "Codex" && c.Group != "OpenRouter" && c.Group != "Currency" {
			t.Fatalf("unexpected widget group %q", c.Group)
		}
	}
	openRouterURLs := 0
	for _, c := range testCatalog(t).Widgets {
		if c.Group == "OpenRouter" {
			if c.Logic.Source == "" || c.Logic.URL == "" || c.Logic.Method == "" {
				t.Fatalf("OpenRouter widget is missing URL logic: %+v", c)
			}
			openRouterURLs++
		}
	}
	if openRouterURLs == 0 {
		t.Fatal("widget catalog has no OpenRouter definitions")
	}
}
func TestWidgetIDsSurviveMissingData(t *testing.T) {
	missing := buildDashboardWithCatalog(snapshot{}, testCatalog(t))
	live := buildDashboardWithCatalog(snapshot{Results: map[string]result{"codex/limits": {Data: fixture(t, `{"buckets":{"codex":{"primary":{"usedPercent":1,"windowDurationMins":300},"individualLimit":{"limit":"50","used":"1","remainingPercent":98,"resetsAt":1700000000}}},"rateLimitResetCredits":{"availableCount":1,"credits":[{"id":"reset-a","title":"A reset","expiresAt":null}]}}`)}}}, testCatalog(t))
	ids := map[string]bool{}
	for _, w := range live.Widgets {
		ids[w.ID] = true
	}
	for _, w := range missing.Widgets {
		if !ids[w.ID] {
			t.Errorf("widget disappeared when data arrived: %s", w.ID)
		}
	}
	if len(ids) <= len(missing.Widgets) {
		t.Fatal("live bucket data did not add the expected dynamic widgets")
	}
	for _, w := range missing.Widgets {
		if !strings.HasPrefix(w.ID, "codex/bucket/") && !ids[w.ID] {
			t.Errorf("static widget disappeared when data arrived: %s", w.ID)
		}
	}
	for _, w := range live.Widgets {
		if w.ID == "codex/resets/details" && (len(w.Resets) != 1 || w.Resets[0].Expires != "No expiration") {
			t.Fatal(w)
		}
	}
}
func TestNullZeroAndAssets(t *testing.T) {
	c := collectorpkg.New(collectorpkg.Options{})
	c.SetSnapshot(snapshot{Results: map[string]result{"codex/limits": {Data: fixture(t, `{"buckets":{"codex":{"credits":{"balance":"<script>alert(1)</script>","hasCredits":false}}},"accountId":"private-account-id"}`)}}})
	w := httptest.NewRecorder()
	newHandlerWithCatalog(c, testCatalog(t)).ServeHTTP(w, httptest.NewRequest("GET", "/api/widgets", nil))
	if strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "private-account-id") {
		t.Fatal("unsafe or unnecessary raw data in widget API")
	}
	if value(nil) != "Unavailable" || value(float64(0)) != "0" || value(false) != "No" {
		t.Fatal("null and zero conflated")
	}
	for _, path := range []string{"/", "/api/widgets", "/assets/dashboard.js", "/assets/dashboard.css", "/assets/gridstack-all.js", "/assets/gridstack.min.css"} {
		for _, method := range []string{"GET", "HEAD", "POST"} {
			w := httptest.NewRecorder()
			newHandlerWithCatalog(c, testCatalog(t)).ServeHTTP(w, httptest.NewRequest(method, path, nil))
			want := 200
			if method == "POST" {
				want = 405
			}
			if w.Code != want {
				t.Fatalf("%s %s: %d", method, path, w.Code)
			}
			if method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
			if method == "GET" && (w.Body.Len() == 0 || w.Header().Get("Content-Type") == "") {
				t.Fatal("missing embedded asset")
			}
		}
	}
	for _, path := range []string{"/missing", "/assets/", "/assets/../codex.go", "/web/vendor/gridstack/"} {
		w := httptest.NewRecorder()
		newHandlerWithCatalog(c, testCatalog(t)).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatal("unexpected asset access", path, w.Code)
		}
	}
}
func TestFailureRetainsSnapshot(t *testing.T) {
	now := time.Now()
	c := collectorpkg.New(collectorpkg.Options{})
	c.SetSnapshot(snapshot{Results: map[string]result{"codex/limits": {Data: map[string]any{"marker": true}, Updated: now}}})
	c.Fail()
	got := c.Snapshot().Results["codex/limits"]
	if got.Data["marker"] != true || !got.Updated.Equal(now) || !strings.Contains(status(got), "Stale") {
		t.Fatal(got)
	}
}

func TestCollectorPartialFailure(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	c := collectorpkg.New(collectorpkg.Options{Providers: []collectorpkg.Reader{
		testReader{name: "codex", read: func(context.Context) map[string]result {
			return map[string]result{
				"codex/account": {Data: map[string]any{"type": "chatgpt"}, Updated: time.Now()},
				"codex/limits":  {Data: map[string]any{"credits": map[string]any{"balance": "0"}}, Updated: time.Now()},
				"codex/usage":   {Error: "unsupported"},
			}
		}},
		testReader{name: "openrouter", read: func(context.Context) map[string]result {
			return map[string]result{"openrouter/key": {Data: map[string]any{"usage_monthly": float64(7)}, Updated: time.Now()}}
		}},
	}})
	c.SetSnapshot(snapshot{Results: map[string]result{"codex/usage": {Data: map[string]any{"old": true}, Updated: old}}})
	c.Refresh(context.Background())
	s := c.Snapshot()
	if s.Results["codex/account"].Error != "" || s.Results["codex/limits"].Error != "" || s.Results["codex/limits"].Updated.IsZero() {
		t.Fatalf("successful reads lost: %+v", s)
	}
	usageResult := s.Results["codex/usage"]
	if usageResult.Error == "" || usageResult.Data["old"] != true || !usageResult.Updated.Equal(old) {
		t.Fatalf("stale activity lost: %+v", usageResult)
	}
	keyResult := s.Results["openrouter/key"]
	if keyResult.Error != "" || keyResult.Data["usage_monthly"] != float64(7) {
		t.Fatalf("OpenRouter refresh was canceled by Codex cleanup: %+v", keyResult)
	}
}
func TestCollectorCancellation(t *testing.T) {
	c := collectorpkg.New(collectorpkg.Options{Providers: []collectorpkg.Reader{testReader{name: "cancel", read: func(ctx context.Context) map[string]result {
		<-ctx.Done()
		return map[string]result{"cancel/status": {Error: ctx.Err().Error()}}
	}}}})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	c.Refresh(ctx)
	if time.Since(start) > 3*time.Second {
		t.Fatal("child process did not stop on cancellation")
	}
	if c.Snapshot().Results["cancel/status"].Error == "" {
		t.Fatal("cancellation not reported")
	}
}
