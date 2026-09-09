// Package providerprotocol defines the wire format used by external providers.
package providerprotocol

import "gryphdash/internal/dashboard"

const Version = 1

type Request struct {
	Version int    `json:"version"`
	Method  string `json:"method"`
}

type Response struct {
	Version  int                         `json:"version"`
	Provider string                      `json:"provider"`
	Widgets  dashboard.WidgetCatalog     `json:"widgets,omitempty"`
	Results  map[string]dashboard.Result `json:"results,omitempty"`
	Error    string                      `json:"error,omitempty"`
}
