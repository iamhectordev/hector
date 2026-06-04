package github

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	gh "github.com/iamhectordev/hector/internal/github"
	"github.com/iamhectordev/hector/internal/mcp"
	"github.com/iamhectordev/hector/modules/tools"
)

// Register wires GitHub tools into registry. Caller must close the returned closer on shutdown.
func Register(ctx context.Context, cfg gh.Config, registry *tools.Registry) (io.Closer, error) {
	tokens, err := gh.NewTokenProvider(cfg)
	if err != nil {
		return nil, err
	}
	client, err := gh.NewClientWithTokenProvider(gh.ClientConfig{APIURL: cfg.APIURL}, tokens)
	if err != nil {
		return nil, err
	}
	toolList, err := NewTools(client)
	if err != nil {
		return nil, err
	}
	for _, t := range toolList {
		if err := registry.Register(t); err != nil {
			return nil, err
		}
	}
	if !cfg.MCP.Configured() {
		return nopCloser{}, nil
	}
	return registerMCP(ctx, cfg, tokens, registry)
}

func registerMCP(ctx context.Context, cfg gh.Config, tokens gh.TokenProvider, registry *tools.Registry) (io.Closer, error) {
	token, err := tokens.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: get token for mcp: %w", err)
	}
	mcpClient, err := mcp.NewClient(mcpClientConfig(cfg.MCP, token))
	if err != nil {
		return nil, err
	}
	if err := mcpClient.Start(ctx); err != nil {
		return nil, err
	}
	var ok bool
	defer func() {
		if !ok {
			_ = mcpClient.Close()
		}
	}()
	discovered, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	for _, discoveredTool := range discovered {
		t, err := tools.NewMCPTool("github", mcpClient, discoveredTool)
		if err != nil {
			return nil, err
		}
		if err := registry.Register(t); err != nil {
			return nil, err
		}
	}
	slog.InfoContext(ctx, "github mcp tools registered", "count", len(discovered))
	ok = true
	return mcpClient, nil
}

func mcpClientConfig(cfg gh.MCPConfig, token gh.AccessToken) mcp.Config {
	transport := mcp.Transport(cfg.Transport)
	if transport == "" {
		switch {
		case cfg.StreamableHTTPURL != "":
			transport = mcp.TransportStreamableHTTP
		case cfg.SSEURL != "":
			transport = mcp.TransportSSE
		default:
			transport = mcp.TransportStdio
		}
	}
	mcpCfg := mcp.Config{Transport: transport}
	mcpCfg.Stdio = mcp.StdioConfig{
		Command: cfg.StdioCommand,
		Args:    append([]string{}, cfg.StdioArgs...),
		Env:     cloneStringMap(cfg.StdioEnv),
	}
	if token.Value != "" {
		if mcpCfg.Stdio.Env == nil {
			mcpCfg.Stdio.Env = map[string]string{}
		}
		if _, exists := mcpCfg.Stdio.Env["GITHUB_PERSONAL_ACCESS_TOKEN"]; !exists {
			mcpCfg.Stdio.Env["GITHUB_PERSONAL_ACCESS_TOKEN"] = token.Value
		}
	}
	if transport == mcp.TransportStreamableHTTP {
		mcpCfg.StreamableHTTP = mcp.StreamableHTTPConfig{
			URL:                  cfg.StreamableHTTPURL,
			Headers:              mcpHTTPHeaders(cfg, token, cfg.StreamableHTTPHeaders),
			DisableStandaloneSSE: cfg.DisableStandaloneSSE,
		}
	}
	if transport == mcp.TransportSSE {
		mcpCfg.SSE = mcp.SSEConfig{
			URL:     cfg.SSEURL,
			Headers: mcpHTTPHeaders(cfg, token, cfg.SSEHeaders),
		}
	}
	return mcpCfg
}

func mcpHTTPHeaders(cfg gh.MCPConfig, token gh.AccessToken, configured map[string]string) map[string]string {
	headers := cloneStringMap(configured)
	if headers == nil {
		headers = map[string]string{}
	}
	if token.Value != "" {
		headers["Authorization"] = "Bearer " + token.Value
	}
	if cfg.Toolsets != "" {
		headers["X-MCP-Toolsets"] = cfg.Toolsets
	}
	if cfg.Readonly {
		headers["X-MCP-Readonly"] = "true"
	}
	return headers
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
