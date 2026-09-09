package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"gryphdash/internal/providerprotocol"
	"gryphdash/providers/currency"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-widgets" {
		_ = json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "currency", Widgets: currency.Catalog()})
		return
	}
	var request providerprotocol.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fail(fmt.Errorf("invalid provider request: %w", err))
	}
	if request.Version != providerprotocol.Version || request.Method != "read" {
		fail(fmt.Errorf("unsupported provider request"))
	}
	pairs := currency.ParsePairs(os.Getenv("GRYPHDASH_CURRENCY_PAIRS"))
	if len(pairs) == 0 {
		pairs = []currency.Pair{{Base: "EUR", Quote: "USD"}, {Base: "EUR", Quote: "GBP"}, {Base: "EUR", Quote: "HUF"}}
	}
	result := currency.Client{HTTPClient: &http.Client{}, BaseURL: os.Getenv("GRYPHDASH_CURRENCY_URL"), Pairs: pairs}.Read(context.Background())
	if err := json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "currency", Results: result}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "currency", Error: err.Error()})
	os.Exit(1)
}
