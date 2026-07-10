package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportStreamableHTTP Transport = "streamable_http"
	TransportSSE            Transport = "sse"
)

// Config selects one MCP transport and carries that transport's config.
type Config struct {
	Transport Transport `yaml:"transport" env:"MCP_TRANSPORT" validate:"required"`

	Stdio          StdioConfig          `yaml:"stdio"`
	StreamableHTTP StreamableHTTPConfig `yaml:"streamable_http"`
	SSE            SSEConfig            `yaml:"sse"`
}

type StdioConfig struct {
	Command string            `yaml:"command" env:"MCP_STDIO_COMMAND"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

type StreamableHTTPConfig struct {
	URL                  string            `yaml:"url" env:"MCP_STREAMABLE_HTTP_URL"`
	Headers              map[string]string `yaml:"headers"`
	DisableStandaloneSSE bool              `yaml:"disable_standalone_sse"`
}

type SSEConfig struct {
	URL     string            `yaml:"url" env:"MCP_SSE_URL"`
	Headers map[string]string `yaml:"headers"`
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

// Client wraps the official MCP SDK behind Hector's small MCP boundary.
type Client struct {
	cfg     Config
	session *sdkmcp.ClientSession
}

// NewClient validates config and returns a client.
func NewClient(cfg Config) (*Client, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg}, nil
}

// Start connects to the configured MCP transport.
func (c *Client) Start(ctx context.Context) error {
	if c.session != nil {
		return nil
	}
	transport, err := c.transport()
	if err != nil {
		return err
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "hector", Version: "dev"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("mcp: connect: %w", err)
	}
	c.session = session
	return nil
}

// Close closes the MCP session.
func (c *Client) Close() error {
	if c.session == nil {
		return nil
	}
	err := c.session.Close()
	c.session = nil
	return err
}

// ListTools returns tools advertised by the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if c.session == nil {
		return nil, fmt.Errorf("mcp: client is not started")
	}
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		inputSchema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("mcp: encode input schema for %q: %w", tool.Name, err)
		}
		tools = append(tools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchema,
		})
	}
	return tools, nil
}

// CallTool calls one MCP tool.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (ToolResult, error) {
	if c.session == nil {
		return ToolResult{}, fmt.Errorf("mcp: client is not started")
	}
	var arguments any
	if len(args) == 0 {
		arguments = map[string]any{}
	} else if err := json.Unmarshal(args, &arguments); err != nil {
		return ToolResult{}, fmt.Errorf("mcp: decode tool arguments: %w", err)
	}
	result, err := c.session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return convertToolResult(result)
}

func (c *Client) transport() (sdkmcp.Transport, error) {
	switch c.cfg.Transport {
	case TransportStdio:
		cmd := exec.Command(c.cfg.Stdio.Command, c.cfg.Stdio.Args...)
		cmd.Env = os.Environ()
		for key, value := range c.cfg.Stdio.Env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
		return &sdkmcp.CommandTransport{Command: cmd}, nil
	case TransportStreamableHTTP:
		return &sdkmcp.StreamableClientTransport{
			Endpoint:             c.cfg.StreamableHTTP.URL,
			HTTPClient:           headerHTTPClient(c.cfg.StreamableHTTP.Headers),
			DisableStandaloneSSE: c.cfg.StreamableHTTP.DisableStandaloneSSE,
		}, nil
	case TransportSSE:
		return &sdkmcp.SSEClientTransport{
			Endpoint:   c.cfg.SSE.URL,
			HTTPClient: headerHTTPClient(c.cfg.SSE.Headers),
		}, nil
	default:
		return nil, fmt.Errorf("mcp: unsupported transport %q", c.cfg.Transport)
	}
}

func validateConfig(cfg Config) error {
	switch cfg.Transport {
	case TransportStdio:
		if cfg.Stdio.Command == "" {
			return fmt.Errorf("mcp: stdio command is required")
		}
	case TransportStreamableHTTP:
		if cfg.StreamableHTTP.URL == "" {
			return fmt.Errorf("mcp: streamable_http url is required")
		}
	case TransportSSE:
		if cfg.SSE.URL == "" {
			return fmt.Errorf("mcp: sse url is required")
		}
	case "":
		return fmt.Errorf("mcp: transport is required")
	default:
		return fmt.Errorf("mcp: unsupported transport %q", cfg.Transport)
	}
	return nil
}

func headerHTTPClient(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return http.DefaultClient
	}
	return &http.Client{Transport: headerRoundTripper{
		headers: headers,
		base:    http.DefaultTransport,
	}}
}

type headerRoundTripper struct {
	headers map[string]string
	base    http.RoundTripper
}

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for key, value := range h.headers {
		clone.Header.Set(key, value)
	}
	return h.base.RoundTrip(clone)
}

func convertToolResult(result *sdkmcp.CallToolResult) (ToolResult, error) {
	if result == nil {
		return ToolResult{}, fmt.Errorf("mcp: nil tool result")
	}
	content := make([]Content, 0, len(result.Content))
	for _, item := range result.Content {
		itemJSON, err := item.MarshalJSON()
		if err != nil {
			return ToolResult{}, fmt.Errorf("mcp: encode content item: %w", err)
		}
		var decoded Content
		if err := json.Unmarshal(itemJSON, &decoded); err != nil {
			return ToolResult{}, fmt.Errorf("mcp: decode content item: %w", err)
		}
		content = append(content, decoded)
	}
	return ToolResult{Content: content, IsError: result.IsError}, nil
}
