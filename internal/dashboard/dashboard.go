package dashboard

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	collectorpkg "gryphdash/internal/collector"
)

//go:embed widgets.json
var widgetConfigData []byte

type Result = collectorpkg.Result
type Snapshot = collectorpkg.Snapshot
type result = collectorpkg.Result
type snapshot = collectorpkg.Snapshot

type widgetLogic struct {
	Type   string `json:"type"`
	Source string `json:"source,omitempty"`
	Path   string `json:"path,omitempty"`
	URL    string `json:"url,omitempty"`
	Method string `json:"method,omitempty"`
}
type widgetConfig struct {
	ID          string      `json:"id"`
	Group       string      `json:"group"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Scope       string      `json:"scope,omitempty"`
	Default     bool        `json:"default"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Logic       widgetLogic `json:"logic"`
}
type widgetCatalog struct {
	Widgets []widgetConfig `json:"widgets"`
}

type day struct {
	Date    string  `json:"date"`
	Tokens  string  `json:"tokens"`
	Percent float64 `json:"percent"`
}
type resetCredit struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	Granted     string `json:"granted"`
	Expires     string `json:"expires"`
}
type widget struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Group    string        `json:"group"`
	Source   string        `json:"source,omitempty"`
	Kind     string        `json:"kind"`
	Value    string        `json:"value"`
	Note     string        `json:"note"`
	Status   string        `json:"status"`
	Default  bool          `json:"default"`
	Width    int           `json:"width"`
	Height   int           `json:"height"`
	Percent  *float64      `json:"percent,omitempty"`
	ResetsAt *int64        `json:"resetsAt,omitempty"`
	Days     []day         `json:"days,omitempty"`
	Resets   []resetCredit `json:"resets,omitempty"`
}
type dashboard struct {
	Widgets     []widget `json:"widgets"`
	LastRefresh string   `json:"lastRefresh,omitempty"`
}

func loadWidgetCatalog() widgetCatalog {
	var catalog widgetCatalog
	if err := json.Unmarshal(widgetConfigData, &catalog); err != nil {
		panic(fmt.Sprintf("invalid widgets.json: %v", err))
	}
	for i, w := range catalog.Widgets {
		if w.ID == "" || w.Group == "" || w.Name == "" || w.Description == "" || w.Logic.Type == "" || w.Width < 1 || w.Height < 2 {
			panic(fmt.Sprintf("invalid widget definition at index %d", i))
		}
	}
	return catalog
}

var configuredWidgetCatalog = loadWidgetCatalog()

type Widget = widget
type Dashboard = dashboard
type WidgetCatalog = widgetCatalog

func Catalog() WidgetCatalog              { return configuredWidgetCatalog }
func BuildDashboard(s Snapshot) Dashboard { return buildDashboard(s) }
func Object(v any) map[string]any         { return object(v) }
func Value(v any) string                  { return value(v) }
func Status(r Result) string              { return status(r) }

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func value(v any) string {
	if v == nil {
		return "Unavailable"
	}
	switch x := v.(type) {
	case bool:
		if x {
			return "Yes"
		}
		return "No"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		if x == "" {
			return "Unavailable"
		}
		return x
	}
	return fmt.Sprint(v)
}
func timestamp(v any) string {
	switch x := v.(type) {
	case float64:
		return time.Unix(int64(x), 0).UTC().Format("2006-01-02 15:04 MST")
	case string:
		t, err := time.Parse(time.RFC3339, x)
		if err == nil {
			return t.UTC().Format("2006-01-02 15:04 MST")
		}
	}
	return "Unavailable"
}
func status(r result) string {
	if r.Updated.IsZero() {
		if r.Error != "" {
			return r.Error
		}
		return "Waiting for first update…"
	}
	text := "Updated " + r.Updated.UTC().Format("2006-01-02 15:04:05 MST")
	if r.Error != "" {
		return "Stale · " + text + " · " + r.Error
	}
	return text
}
func pathValue(root map[string]any, path string) any {
	var current any = root
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}
func sourceResult(logic widgetLogic, s snapshot) result {
	switch logic.Source {
	case "account":
		return s.Account
	case "limits":
		return s.Limits
	case "usage":
		return s.Usage
	case "openrouterKey":
		return s.OpenRouterKey
	case "openrouterCredits":
		return s.OpenRouterCredits
	}
	return result{}
}
func configWidget(c widgetConfig, id, group string, raw any, r result) widget {
	return widget{ID: id, Title: c.Name, Group: group, Source: c.Logic.Source, Kind: c.Logic.Type, Value: value(raw), Note: c.Description, Status: status(r), Default: c.Default, Width: c.Width, Height: c.Height}
}
func limitWidget(c widgetConfig, id, group string, raw any, r result) widget {
	w := configWidget(c, id, group, nil, r)
	m := object(raw)
	if m == nil {
		w.Note = c.Description + " · Window not returned"
		return w
	}
	if mins, ok := m["windowDurationMins"].(float64); ok {
		switch mins {
		case 300:
			w.Title = "5-hour limit"
		case 10080:
			w.Title = "Weekly limit"
		default:
			w.Title = fmt.Sprintf("%g-minute limit", mins)
		}
	}
	if used, ok := m["usedPercent"].(float64); ok {
		remaining := 100 - used
		w.Value = fmt.Sprintf("%g%%", remaining)
		w.Percent = &remaining
		w.Note = fmt.Sprintf("%s · %g%% used", c.Description, used)
	} else {
		w.Note = c.Description + " · Usage percentage unavailable"
	}
	if seconds, ok := m["resetsAt"].(float64); ok {
		n := int64(seconds)
		w.ResetsAt = &n
	} else {
		w.Note += " · Reset unavailable"
	}
	return w
}
func staticWidget(c widgetConfig, s snapshot) widget {
	r := sourceResult(c.Logic, s)
	raw := pathValue(r.Data, c.Logic.Path)
	w := configWidget(c, c.ID, c.Group, raw, r)
	switch c.Logic.Type {
	case "resetDetails":
		w.Kind = "resets"
		w.Width = 8
		w.Height = 6
		resets := object(raw)
		rows, _ := resets["credits"].([]any)
		w.Value = fmt.Sprintf("%d reset details returned", len(rows))
		for _, row := range rows {
			m := object(row)
			expires := timestamp(m["expiresAt"])
			if m["expiresAt"] == nil {
				expires = "No expiration"
			}
			w.Resets = append(w.Resets, resetCredit{value(m["id"]), value(m["title"]), value(m["description"]), value(m["status"]), value(m["resetType"]), timestamp(m["grantedAt"]), expires})
		}
	case "daily":
		w.Kind = "daily"
		w.Width = 8
		w.Height = 6
		rows, _ := raw.([]any)
		w.Value = fmt.Sprintf("%d days returned", len(rows))
		max := float64(0)
		for _, row := range rows {
			n, _ := object(row)["tokens"].(float64)
			if n > max {
				max = n
			}
		}
		for _, row := range rows {
			m := object(row)
			n, _ := m["tokens"].(float64)
			pct := float64(0)
			if max > 0 {
				pct = 100 * n / max
			}
			w.Days = append(w.Days, day{value(m["startDate"]), value(m["tokens"]), pct})
		}
		sort.Slice(w.Days, func(i, j int) bool { return w.Days[i].Date > w.Days[j].Date })
	case "timestamp":
		w.Value = timestamp(raw)
	}
	return w
}
func bucketID(template, bucket string) string {
	return strings.ReplaceAll(template, "{bucket}", url.PathEscape(bucket))
}
func buildDashboard(s snapshot) dashboard {
	out := dashboard{Widgets: []widget{}}
	lastRefresh := s.LastRefresh
	for _, r := range []result{s.Account, s.Limits, s.Usage, s.OpenRouterKey, s.OpenRouterCredits} {
		if r.Updated.After(lastRefresh) {
			lastRefresh = r.Updated
		}
	}
	if !lastRefresh.IsZero() {
		out.LastRefresh = lastRefresh.UTC().Format(time.RFC3339)
	}
	buckets := object(s.Limits.Data["rateLimitsByLimitId"])
	if len(buckets) == 0 {
		buckets = map[string]any{"codex": s.Limits.Data["rateLimits"]}
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, c := range configuredWidgetCatalog.Widgets {
		if c.Scope == "limitBuckets" {
			for _, key := range keys {
				b := object(buckets[key])
				copy := c
				copy.Default = c.Default && key == "codex"
				group := c.Group
				id := bucketID(c.ID, key)
				raw := pathValue(b, c.Logic.Path)
				if c.Logic.Type == "limitWindow" {
					out.Widgets = append(out.Widgets, limitWidget(copy, id, group, raw, s.Limits))
				} else {
					out.Widgets = append(out.Widgets, configWidget(copy, id, group, raw, s.Limits))
				}
			}
			continue
		}
		out.Widgets = append(out.Widgets, staticWidget(c, s))
	}
	return out
}
