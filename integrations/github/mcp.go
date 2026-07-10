package github

import (
	"github.com/iamhectordev/hector/pkg/mcp"
)

func mcpClientConfig(cfg MCPConfig, token AccessToken) mcp.Config {
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

func mcpHTTPHeaders(cfg MCPConfig, token AccessToken, configured map[string]string) map[string]string {
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
