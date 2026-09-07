package codex

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestRPCClientSkipsNotifications(t *testing.T) {
	rpc := rpcClient{input: &bytes.Buffer{}, output: bufio.NewScanner(strings.NewReader("{\"method\":\"account/updated\"}\n{\"id\":1,\"result\":{\"ok\":true}}\n"))}
	got, err := rpc.call("account/read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("result: %#v", got)
	}
}

func TestRPCClientErrorsAreSanitized(t *testing.T) {
	rpc := rpcClient{input: &bytes.Buffer{}, output: bufio.NewScanner(strings.NewReader("{\"id\":1,\"error\":{\"code\":-32601,\"message\":\"secret\"}}\n"))}
	_, err := rpc.call("account/read", nil)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}
