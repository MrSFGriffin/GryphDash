package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	limits := fixture(t, `{"rateLimitsByLimitId":{"codex":{"primary":{"usedPercent":25,"windowDurationMins":300,"resetsAt":1700003600},"secondary":{"usedPercent":4,"windowDurationMins":10080},"credits":{"balance":"0","hasCredits":false,"unlimited":false}},"other":{"primary":{"usedPercent":10,"windowDurationMins":60}}},"rateLimitResetCredits":{"availableCount":2,"credits":[]}}`)
	usage := fixture(t, `{"summary":{"lifetimeTokens":123,"currentStreakDays":0},"dailyUsageBuckets":[{"startDate":"2026-01-01","tokens":0},{"startDate":"2026-01-02","tokens":50}]}`)
	c := &collector{state: snapshot{Limits: result{Data: limits, Updated: now}, Usage: result{Data: usage, Updated: now}}}
	w := httptest.NewRecorder()
	newHandler(c).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, want := range []string{"5-hour limit", "Weekly limit", "75% remaining", "96% remaining", "Codex · other", "Credits remaining", "Available resets", "Lifetime tokens", "123", "2026-01-02", "Unavailable", "fictional examples"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
	data := buildDashboard(c.snapshot(), now)
	var days []day
	for _, p := range data.Providers {
		if p.Name == "Token activity" {
			days = p.Days
		}
	}
	if len(days) != 2 || days[0].Percent != 100 || days[1].Percent != 0 {
		t.Fatalf("bad daily chart: %+v", days)
	}
}
func TestNullZeroAndEscaping(t *testing.T) {
	c := &collector{state: snapshot{Limits: result{Data: fixture(t, `{"rateLimits":{"credits":{"balance":"<script>alert(1)</script>","hasCredits":false}}}`)}}}
	w := httptest.NewRecorder()
	newHandler(c).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), "<script>") {
		t.Fatal("unescaped provider value")
	}
	if value(nil) != "Unavailable" || value(float64(0)) != "0" || value(false) != "No" {
		t.Fatal("null and zero conflated")
	}
	for _, tc := range []struct {
		method, path string
		code         int
	}{{"POST", "/", 405}, {"GET", "/missing", 404}, {"HEAD", "/", 200}} {
		w := httptest.NewRecorder()
		newHandler(c).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.code {
			t.Fatal(tc, w.Code)
		}
		if tc.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
}
func TestRPCNotificationsAndErrors(t *testing.T) {
	var input bytes.Buffer
	r := rpcClient{input: &input, output: bufio.NewScanner(strings.NewReader("{\"method\":\"account/updated\"}\n{\"id\":1,\"result\":{\"ok\":true}}\n{\"id\":2,\"error\":{\"code\":-32601,\"message\":\"secret\"}}\n"))}
	data, err := r.call("account/read", nil)
	if err != nil || data["ok"] != true {
		t.Fatal(data, err)
	}
	_, err = r.call("account/usage/read", nil)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("missing or unsafe error", err)
	}
}
func TestFailureRetainsSnapshot(t *testing.T) {
	now := time.Now()
	c := &collector{state: snapshot{Limits: result{Data: map[string]any{"marker": true}, Updated: now}}}
	c.fail()
	got := c.snapshot().Limits
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
	old := time.Now().Add(-time.Hour)
	c := &collector{executable: path, state: snapshot{Usage: result{Data: map[string]any{"old": true}, Updated: old}}}
	c.refresh(context.Background())
	s := c.snapshot()
	if s.Account.Error != "" || s.Limits.Error != "" || s.Limits.Updated.IsZero() {
		t.Fatalf("successful reads lost: %+v", s)
	}
	if s.Usage.Error == "" || s.Usage.Data["old"] != true || !s.Usage.Updated.Equal(old) {
		t.Fatalf("stale activity lost: %+v", s.Usage)
	}
}
func TestCollectorCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake-codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nwhile IFS= read -r line; do :; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	c := &collector{executable: path}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	c.refresh(ctx)
	if time.Since(start) > 3*time.Second {
		t.Fatal("child process did not stop on cancellation")
	}
	if c.snapshot().Limits.Error == "" {
		t.Fatal("cancellation not reported")
	}
}
