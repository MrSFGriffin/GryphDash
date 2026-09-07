package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

type result struct {
	Data    map[string]any
	Updated time.Time
	Error   string
}
type snapshot struct {
	Account, Limits, Usage           result
	OpenRouterKey, OpenRouterCredits result
}
type collector struct {
	mu            sync.RWMutex
	state         snapshot
	executable    string
	openRouterKey string
	httpClient    *http.Client
}

func (c *collector) snapshot() snapshot { c.mu.RLock(); defer c.mu.RUnlock(); return c.state }

type rpcClient struct {
	input  io.Writer
	output *bufio.Scanner
	next   int
}

func (r *rpcClient) call(method string, params any) (map[string]any, error) {
	r.next++
	id := r.next
	if err := json.NewEncoder(r.input).Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for r.output.Scan() {
		var msg struct {
			ID     *int           `json:"id"`
			Method string         `json:"method"`
			Result map[string]any `json:"result"`
			Error  *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(r.output.Bytes(), &msg); err != nil {
			return nil, fmt.Errorf("invalid app-server response")
		}
		if msg.Method != "" {
			if msg.ID != nil { // No server-initiated actions are supported by this read-only client.
				if err := json.NewEncoder(r.input).Encode(map[string]any{"id": *msg.ID, "error": map[string]any{"code": -32601, "message": "Unsupported method"}}); err != nil {
					return nil, err
				}
			}
			continue
		}
		if msg.ID == nil || *msg.ID != id {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("app-server error %d; check Codex login and CLI version", msg.Error.Code)
		}
		if msg.Result == nil {
			return nil, fmt.Errorf("empty app-server result")
		}
		return msg.Result, nil
	}
	if err := r.output.Err(); err != nil {
		return nil, fmt.Errorf("reading app-server response: %w", err)
	}
	return nil, fmt.Errorf("app-server stopped or request timed out")
}
func (c *collector) refresh(parent context.Context) {
	openRouterDone := make(chan struct{})
	go func() {
		defer close(openRouterDone)
		c.refreshOpenRouter(parent)
	}()
	defer func() { <-openRouterDone }()
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.executable, "app-server")
	input, err := cmd.StdinPipe()
	if err != nil {
		c.fail()
		return
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		c.fail()
		return
	}
	defer output.Close()
	// Discard stderr: upstream diagnostics may contain account information.
	if err = cmd.Start(); err != nil {
		c.fail()
		return
	}
	defer func() { input.Close(); cancel(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	rpc := &rpcClient{input: input, output: scanner}
	_, err = rpc.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "gryphdash", "version": "0.2.0"}})
	if err != nil {
		c.fail()
		return
	}
	if err = json.NewEncoder(input).Encode(map[string]any{"method": "initialized"}); err != nil {
		c.fail()
		return
	}
	for _, method := range []string{"account/read", "account/rateLimits/read", "account/usage/read"} {
		data, err := rpc.call(method, nil)
		c.mu.Lock()
		target := &c.state.Account
		if method == "account/rateLimits/read" {
			target = &c.state.Limits
		}
		if method == "account/usage/read" {
			target = &c.state.Usage
		}
		if err != nil {
			target.Error = err.Error()
		} else {
			*target = result{Data: data, Updated: time.Now()}
		}
		c.mu.Unlock()
	}
}
func (c *collector) fail() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range []*result{&c.state.Account, &c.state.Limits, &c.state.Usage, &c.state.OpenRouterKey, &c.state.OpenRouterCredits} {
		r.Error = "Could not connect to Codex. Check that the CLI is installed and run codex login as the server user."
	}
}
func (c *collector) run(ctx context.Context, interval time.Duration) {
	for {
		c.refresh(ctx)
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
