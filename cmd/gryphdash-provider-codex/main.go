package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gryphdash/internal/providerprotocol"
	"gryphdash/providers/codex"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-widgets" {
		_ = json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "codex", Widgets: codex.Catalog()})
		return
	}
	var request providerprotocol.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fail(fmt.Errorf("invalid provider request: %w", err))
	}
	if request.Version != providerprotocol.Version || request.Method != "read" {
		fail(fmt.Errorf("unsupported provider request"))
	}
	adapter := codex.NewAdapter(os.Getenv("GRYPHDASH_CODEX_BIN"))
	if os.Getenv("GRYPHDASH_CODEX_BIN") == "" {
		adapter = codex.NewAdapter("codex")
	}
	results := adapter.Read(context.Background())
	if err := json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "codex", Results: results}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(providerprotocol.Response{Version: providerprotocol.Version, Provider: "codex", Error: err.Error()})
	os.Exit(1)
}
