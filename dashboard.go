package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

type metric struct{ Label, Value, Note string }
type day struct {
	Date    string
	Tokens  string
	Percent float64
}
type provider struct {
	Name, Detail, Status string
	Metrics              []metric
	Days                 []day
}
type dashboard struct{ Providers []provider }

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
		return fmt.Sprintf("%g", x)
	case string:
		if x == "" {
			return "Unavailable"
		}
		return x
	}
	return fmt.Sprint(v)
}
func label(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String()[:1]) + b.String()[1:]
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
func fields(m map[string]any, prefix string) []metric {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []metric
	for _, k := range keys {
		name := prefix + label(k)
		if nested, ok := m[k].(map[string]any); ok {
			out = append(out, fields(nested, name+" · ")...)
			continue
		}
		if rows, ok := m[k].([]any); ok {
			if len(rows) == 0 {
				out = append(out, metric{name, "None returned", ""})
			}
			for i, row := range rows {
				out = append(out, fields(object(row), fmt.Sprintf("%s %d · ", name, i+1))...)
			}
			continue
		}
		v := value(m[k])
		if strings.HasSuffix(k, "At") {
			v = timestamp(m[k])
		}
		if k == "expiresAt" && m[k] == nil {
			v = "No expiration"
		}
		out = append(out, metric{name, v, ""})
	}
	return out
}
func window(name string, v any, now time.Time) metric {
	m := object(v)
	if m == nil {
		return metric{name, "Unavailable", "Window not returned"}
	}
	if mins, ok := m["windowDurationMins"].(float64); ok {
		switch mins {
		case 300:
			name = "5-hour limit"
		case 10080:
			name = "Weekly limit"
		default:
			name = fmt.Sprintf("%g-minute limit", mins)
		}
	}
	display := "Unavailable"
	if used, ok := m["usedPercent"].(float64); ok {
		display = fmt.Sprintf("%g%% remaining", 100-used)
	}
	note := "Used: " + value(m["usedPercent"]) + "% · Reset: " + timestamp(m["resetsAt"])
	if seconds, ok := m["resetsAt"].(float64); ok {
		left := time.Unix(int64(seconds), 0).Sub(now)
		if left > 0 {
			note += " · in " + left.Round(time.Minute).String()
		} else {
			note += " · awaiting updated window"
		}
	}
	return metric{name, display, note}
}
func buildDashboard(s snapshot, now time.Time) dashboard {
	account := object(s.Account.Data["account"])
	cards := []provider{{Name: "Codex account", Detail: "Live account", Status: status(s.Account), Metrics: []metric{{"Plan", value(account["planType"]), ""}, {"Authentication", value(account["type"]), ""}}}}
	buckets := object(s.Limits.Data["rateLimitsByLimitId"])
	if len(buckets) == 0 {
		buckets = map[string]any{"codex": s.Limits.Data["rateLimits"]}
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := object(buckets[k])
		card := provider{Name: "Codex · " + k, Detail: "Usage limits & credits", Status: status(s.Limits)}
		card.Metrics = []metric{window("Primary limit", b["primary"], now), window("Secondary limit", b["secondary"], now)}
		credits := object(b["credits"])
		card.Metrics = append(card.Metrics, metric{"Credits remaining", value(credits["balance"]), "Credits, not USD"}, metric{"Credits available", value(credits["hasCredits"]), ""}, metric{"Unlimited credits", value(credits["unlimited"]), ""})
		extra := map[string]any{"planType": b["planType"], "limitName": b["limitName"], "individualLimit": b["individualLimit"], "spendControlReached": b["spendControlReached"], "rateLimitReachedType": b["rateLimitReachedType"]}
		card.Metrics = append(card.Metrics, fields(extra, "")...)
		cards = append(cards, card)
	}
	resets := object(s.Limits.Data["rateLimitResetCredits"])
	resetCard := provider{Name: "Earned resets", Detail: "Read only · reset credits are separate from spendable credits", Status: status(s.Limits), Metrics: []metric{{"Available resets", value(resets["availableCount"]), "The provider may return only some reset details"}}}
	resetCard.Metrics = append(resetCard.Metrics, fields(map[string]any{"credits": resets["credits"]}, "")...)
	cards = append(cards, resetCard)
	summary := object(s.Usage.Data["summary"])
	usage := provider{Name: "Token activity", Detail: "Account activity returned by Codex", Status: status(s.Usage)}
	for _, item := range []struct{ key, name, unit string }{{"lifetimeTokens", "Lifetime tokens", "tokens"}, {"peakDailyTokens", "Peak daily tokens", "tokens"}, {"longestRunningTurnSec", "Longest-running turn", "seconds"}, {"currentStreakDays", "Current streak", "days"}, {"longestStreakDays", "Longest streak", "days"}} {
		usage.Metrics = append(usage.Metrics, metric{item.name, value(summary[item.key]), item.unit})
	}
	rows, ok := s.Usage.Data["dailyUsageBuckets"].([]any)
	if !ok {
		usage.Metrics = append(usage.Metrics, metric{"Daily activity", "Unavailable", ""})
	} else if len(rows) == 0 {
		usage.Metrics = append(usage.Metrics, metric{"Daily activity", "No activity returned", ""})
	}
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
		usage.Days = append(usage.Days, day{value(m["startDate"]), value(m["tokens"]), pct})
	}
	sort.Slice(usage.Days, func(i, j int) bool { return usage.Days[i].Date > usage.Days[j].Date })
	cards = append(cards, usage, provider{Name: "OpenRouter", Detail: "Demo data · USD", Status: "Not connected · fictional examples", Metrics: []metric{{"Spend this month", "$12.40", "Example monthly budget: $50.00"}, {"Budget remaining", "$37.60", "75.2% of example monthly budget"}, {"Credit balance", "$87.60", "Example prepaid credits"}}})
	return dashboard{cards}
}
