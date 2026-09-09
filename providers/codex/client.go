package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

type Result struct {
	Data    map[string]any
	Updated time.Time
	Error   string
}
type Data struct{ Account, Limits, Usage Result }
type Client struct{ Executable string }
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
			if msg.ID != nil {
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

func (c Client) Read(parent context.Context) Data {
	out := Data{}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Executable, "app-server")
	configureCommand(cmd)
	input, err := cmd.StdinPipe()
	if err != nil {
		out.fail(err)
		return out
	}
	defer input.Close()
	output, err := cmd.StdoutPipe()
	if err != nil {
		out.fail(err)
		return out
	}
	defer output.Close()
	if err = cmd.Start(); err != nil {
		out.fail(err)
		return out
	}
	defer func() { input.Close(); cancel(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	rpc := &rpcClient{input: input, output: scanner}
	if _, err = rpc.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "gryphdash", "version": "0.2.0"}}); err != nil {
		out.fail(err)
		return out
	}
	if err = json.NewEncoder(input).Encode(map[string]any{"method": "initialized"}); err != nil {
		out.fail(err)
		return out
	}
	for _, method := range []string{"account/read", "account/rateLimits/read", "account/usage/read"} {
		data, callErr := rpc.call(method, nil)
		r := &out.Account
		if method == "account/rateLimits/read" {
			r = &out.Limits
		}
		if method == "account/usage/read" {
			r = &out.Usage
		}
		if callErr != nil {
			r.Error = callErr.Error()
		} else {
			*r = Result{Data: data, Updated: time.Now()}
		}
	}
	return out
}
func (d *Data) fail(err error) {
	message := "Could not connect to Codex. Check that the CLI is installed and run codex login as the server user."
	if err != nil {
		message = err.Error()
	}
	d.Account.Error, d.Limits.Error, d.Usage.Error = message, message, message
}
