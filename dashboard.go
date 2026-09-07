package main

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"
)

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

// IDs describe the source field, not its current label, value, or list position.
// Browser layouts can therefore survive missing data and quota-window changes.
type widget struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Group    string        `json:"group"`
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
	Widgets []widget `json:"widgets"`
}

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
	n, ok := v.(float64)
	if !ok {
		return "Unavailable"
	}
	return time.Unix(int64(n), 0).UTC().Format("2006-01-02 15:04 MST")
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
func scalar(id, title, group string, v any, note string, r result, visible bool) widget {
	return widget{ID: id, Title: title, Group: group, Kind: "metric", Value: value(v), Note: note, Status: status(r), Default: visible, Width: 4, Height: 4}
}
func limitWidget(id, title, group string, v any, r result, visible bool) widget {
	m := object(v)
	w := scalar(id, title, group, nil, "Window not returned", r, visible)
	if m == nil {
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
		w.Note = fmt.Sprintf("Remaining · %g%% used", used)
	} else {
		w.Note = "Usage percentage unavailable"
	}
	if seconds, ok := m["resetsAt"].(float64); ok {
		n := int64(seconds)
		w.ResetsAt = &n
	} else {
		w.Note += " · Reset unavailable"
	}
	return w
}
func buildDashboard(s snapshot) dashboard {
	out := dashboard{Widgets: []widget{}}
	add := func(w widget) { out.Widgets = append(out.Widgets, w) }
	buckets := object(s.Limits.Data["rateLimitsByLimitId"])
	if len(buckets) == 0 {
		buckets = map[string]any{"codex": s.Limits.Data["rateLimits"]}
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		b := object(buckets[key])
		prefix := "codex/bucket/" + url.PathEscape(key) + "/"
		group := "Codex · " + key
		primary := key == "codex"
		add(limitWidget(prefix+"primary", "Primary limit", group, b["primary"], s.Limits, primary))
		add(limitWidget(prefix+"secondary", "Secondary limit", group, b["secondary"], s.Limits, primary))
		credits := object(b["credits"])
		add(scalar(prefix+"credits/balance", "Credits remaining", group, credits["balance"], "Spendable credits · not USD", s.Limits, primary))
		add(scalar(prefix+"credits/hasCredits", "Credits available", group, credits["hasCredits"], "Whether the account has spendable credits", s.Limits, false))
		add(scalar(prefix+"credits/unlimited", "Unlimited credits", group, credits["unlimited"], "Provider-reported credit status", s.Limits, false))
		for _, f := range []struct{ key, title string }{{"planType", "Bucket plan"}, {"limitName", "Limit name"}, {"spendControlReached", "Spend control reached"}, {"rateLimitReachedType", "Reached-limit state"}} {
			v := b[f.key]
			if f.key == "rateLimitReachedType" && b != nil && v == nil {
				v = "None reported"
			}
			add(scalar(prefix+f.key, f.title, group, v, "", s.Limits, false))
		}
		individual := object(b["individualLimit"])
		for _, f := range []struct{ key, title, note string }{{"limit", "Individual spend allowance", "Provider-reported units"}, {"used", "Individual spend used", "Provider-reported units"}, {"remainingPercent", "Individual spend remaining", "Percent"}, {"resetsAt", "Individual spend reset", "UTC"}} {
			var v any = individual[f.key]
			if f.key == "resetsAt" {
				v = timestamp(v)
			}
			add(scalar(prefix+"individual/"+f.key, f.title, group, v, f.note, s.Limits, false))
		}
	}
	resets := object(s.Limits.Data["rateLimitResetCredits"])
	add(scalar("codex/resets/count", "Available resets", "Earned resets", resets["availableCount"], "Earned resets are separate from spendable credits", s.Limits, true))
	detail := scalar("codex/resets/details", "Earned reset details", "Earned resets", nil, "Read only · the provider may return fewer details than the available count", s.Limits, false)
	detail.Kind = "resets"
	detail.Width = 8
	detail.Height = 6
	if rows, ok := resets["credits"].([]any); ok {
		detail.Value = fmt.Sprintf("%d reset details returned", len(rows))
		for _, row := range rows {
			m := object(row)
			expires := timestamp(m["expiresAt"])
			if m["expiresAt"] == nil {
				expires = "No expiration"
			}
			detail.Resets = append(detail.Resets, resetCredit{value(m["id"]), value(m["title"]), value(m["description"]), value(m["status"]), value(m["resetType"]), timestamp(m["grantedAt"]), expires})
		}
	}
	add(detail)
	summary := object(s.Usage.Data["summary"])
	for _, f := range []struct{ key, title, unit string }{{"lifetimeTokens", "Lifetime tokens", "tokens"}, {"peakDailyTokens", "Peak daily tokens", "tokens"}, {"longestRunningTurnSec", "Longest-running turn", "seconds"}, {"currentStreakDays", "Current streak", "days"}, {"longestStreakDays", "Longest streak", "days"}} {
		add(scalar("codex/usage/"+f.key, f.title, "Token activity", summary[f.key], f.unit, s.Usage, f.key == "lifetimeTokens" || f.key == "currentStreakDays"))
	}
	daily := scalar("codex/usage/daily", "Daily token activity", "Token activity", nil, "Dates as returned by Codex; missing dates are not filled with zero", s.Usage, true)
	daily.Kind = "daily"
	daily.Width = 8
	daily.Height = 6
	if rows, ok := s.Usage.Data["dailyUsageBuckets"].([]any); ok {
		daily.Value = fmt.Sprintf("%d days returned", len(rows))
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
			daily.Days = append(daily.Days, day{value(m["startDate"]), value(m["tokens"]), pct})
		}
		sort.Slice(daily.Days, func(i, j int) bool { return daily.Days[i].Date > daily.Days[j].Date })
	}
	add(daily)
	account := object(s.Account.Data["account"])
	add(scalar("codex/account/plan", "Account plan", "Account", account["planType"], "ChatGPT subscription", s.Account, true))
	add(scalar("codex/account/auth", "Authentication", "Account", account["type"], "Local Codex login", s.Account, false))
	return out
}
