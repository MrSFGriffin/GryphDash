package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"gryphdash/internal/providerprotocol"
	"gryphdash/providers/openrouter"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-widgets" {
		_ = json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "openrouter", Widgets: openrouter.Catalog()})
		return
	}
	var request providerprotocol.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fail(fmt.Errorf("invalid provider request: %w", err))
	}
	if request.Version != providerprotocol.Version || request.Method != "read" {
		fail(fmt.Errorf("unsupported provider request"))
	}
	results := openrouter.NewAdapter(os.Getenv("OPENROUTER_API_KEY"), &http.Client{}).Read(context.Background())
	if err := json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "openrouter", Results: results}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "openrouter", Error: err.Error()})
	os.Exit(1)
}
