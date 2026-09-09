package codex

import (
	"context"

	"gryphdash/internal/metrics"
)

type Adapter struct{ executable string }

func NewAdapter(executable string) Adapter { return Adapter{executable: executable} }
func (Adapter) Name() string               { return "codex" }

func (a Adapter) Read(ctx context.Context) map[string]metrics.Result {
	data := (Client{Executable: a.executable}).Read(ctx)
	return map[string]metrics.Result{
		"codex/account": {Data: data.Account.Data, Updated: data.Account.Updated, Error: data.Account.Error},
		"codex/limits":  {Data: normalizeLimits(data.Limits.Data), Updated: data.Limits.Updated, Error: data.Limits.Error},
		"codex/usage":   {Data: data.Usage.Data, Updated: data.Usage.Updated, Error: data.Usage.Error},
	}
}

func normalizeLimits(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	if buckets, ok := data["rateLimitsByLimitId"].(map[string]any); ok && len(buckets) > 0 {
		data["buckets"] = buckets
		return data
	}
	if buckets, ok := data["rateLimits"].(map[string]any); ok && len(buckets) > 0 {
		data["buckets"] = map[string]any{"codex": buckets}
	}
	return data
}
