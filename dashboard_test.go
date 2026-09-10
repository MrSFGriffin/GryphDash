package main

import (
	"context"
	"encoding/json"
	openrouterprovider "gryphdash/providers/openrouter"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	collectorpkg "gryphdash/internal/collector"
	dashboardpkg "gryphdash/internal/dashboard"
	codexprovider "gryphdash/providers/codex"
)

func testCatalog(t *testing.T) widgetCatalog {
	t.Helper()
	catalog, err := dashboardpkg.MergeCatalog(dashboardpkg.Catalog(), codexprovider.Catalog(), openrouterprovider.Catalog())
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
	path := filepath.Join(t.TempDir(), "fake-codex")
	script := `#!/bin/sh
while IFS= read -r line; do
 case "$line" in
  *'"method":"initialize"'*) echo '{"id":1,"result":{}}' ;;
  *'"method":"account/read"'*) echo '{"id":2,"result":{"account":{"type":"chatgpt","planType":"plus"}}}' ;;
  *'"method":"account/rateLimits/read"'*) echo '{"id":3,"result":{"rateLimits":{"credits":{"balance":"0"}}}}' ;;
  *'"method":"account/usage/read"'*) echo '{"id":4,"error":{"code":-32601,"message":"unsupported"}}' ;;
 esac
done
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	openRouter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"usage_monthly":7}}`))
	}))
	defer openRouter.Close()
	previousBase := openrouterprovider.BaseURLOverride
	openrouterprovider.BaseURLOverride = openRouter.URL
	defer func() { openrouterprovider.BaseURLOverride = previousBase }()
	old := time.Now().Add(-time.Hour)
	c := collectorpkg.New(collectorpkg.Options{Providers: []collectorpkg.Reader{codexprovider.NewAdapter(path), openrouterprovider.NewAdapter("test-key", openRouter.Client())}})
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
	path := filepath.Join(t.TempDir(), "fake-codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nwhile IFS= read -r line; do :; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c := collectorpkg.New(collectorpkg.Options{Providers: []collectorpkg.Reader{codexprovider.NewAdapter(path)}})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	c.Refresh(ctx)
	if time.Since(start) > 3*time.Second {
		t.Fatal("child process did not stop on cancellation")
	}
	if c.Snapshot().Results["codex/limits"].Error == "" {
		t.Fatal("cancellation not reported")
	}
}
