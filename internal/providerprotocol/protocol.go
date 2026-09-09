// Package providerprotocol defines the wire format used by external providers.
package providerprotocol

import "gryphdash/internal/metrics"

const Version = 1

type Request struct {
	Version int    `json:"version"`
	Method  string `json:"method"`
}

type Response struct {
	Version  int                       `json:"version"`
	Provider string                    `json:"provider"`
	Results  map[string]metrics.Result `json:"results,omitempty"`
	Error    string                    `json:"error,omitempty"`
}
