package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

// Config describes a stdio MCP server process.
type Config struct {
	Command string            `yaml:"command" env:"MCP_COMMAND" validate:"required"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// Tool describes a tool advertised by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolResult is the result of an MCP tools/call request.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

// Content is one MCP result content item.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Client talks to one stdio MCP server process.
type Client struct {
	command string
	args    []string
	env     map[string]string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu     sync.Mutex
	nextID int64
}

// NewClient validates config and returns a client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("mcp: command is required")
	}
	return &Client{
		command: cfg.Command,
		args:    append([]string{}, cfg.Args...),
		env:     cloneEnv(cfg.Env),
		nextID:  1,
	}, nil
}

// Start starts the MCP server process.
func (c *Client) Start(context.Context) error {
	if c.cmd != nil {
		return nil
	}
	cmd := exec.Command(c.command, c.args...)
	cmd.Env = os.Environ()
	for key, value := range c.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mcp: start server: %w", err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)
	return nil
}

// Close stops the MCP server process.
func (c *Client) Close() error {
	if c.cmd == nil {
		return nil
	}
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	err := c.cmd.Wait()
	c.cmd = nil
	c.stdin = nil
	c.stdout = nil
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
		return err
	}
	return nil
}

// ListTools returns tools advertised by the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var response struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", map[string]any{}, &response); err != nil {
		return nil, err
	}
	return response.Tools, nil
}

// CallTool calls one MCP tool.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (ToolResult, error) {
	var response ToolResult
	params := map[string]any{
		"name":      name,
		"arguments": json.RawMessage(args),
	}
	if err := c.call(ctx, "tools/call", params, &response); err != nil {
		return ToolResult{}, err
	}
	return response, nil
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	if c.cmd == nil {
		return fmt.Errorf("mcp: client is not started")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if err := c.write(request); err != nil {
		return err
	}
	type rpcError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	var envelope struct {
		ID     int64           `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := c.read(ctx, &envelope); err != nil {
		return err
	}
	if envelope.ID != id {
		return fmt.Errorf("mcp: response id %d did not match request id %d", envelope.ID, id)
	}
	if envelope.Error != nil {
		return fmt.Errorf("mcp: %s failed: %s", method, envelope.Error.Message)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("mcp: decode %s result: %w", method, err)
	}
	return nil
}

func (c *Client) write(message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("mcp: encode request: %w", err)
	}
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		return fmt.Errorf("mcp: write request: %w", err)
	}
	return nil
}

func (c *Client) read(_ context.Context, out any) error {
	headers, err := textproto.NewReader(c.stdout).ReadMIMEHeader()
	if err != nil {
		return fmt.Errorf("mcp: read response headers: %w", err)
	}
	length, err := strconv.Atoi(headers.Get("Content-Length"))
	if err != nil {
		return fmt.Errorf("mcp: invalid content length: %w", err)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return fmt.Errorf("mcp: read response body: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("mcp: decode response: %w", err)
	}
	return nil
}

func cloneEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = value
	}
	return out
}
