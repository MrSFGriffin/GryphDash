package metrics

import "time"

// Result is a provider source result. Providers own the meaning of Data and
// the source names under which results are published.
type Result struct {
	Data    map[string]any
	Updated time.Time
	Error   string
}

type Snapshot struct {
	Results     map[string]Result
	LastRefresh time.Time
}
