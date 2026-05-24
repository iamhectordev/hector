package github

import (
	"fmt"
	"net/url"

	"github.com/go-playground/validator/v10"
	"github.com/iamhectordev/hector/internal/mcp"
)

const defaultAPIURL = "https://api.github.com"

var validate = validator.New(validator.WithRequiredStructEnabled())

type AuthType string

const AuthTypeAppInstallation AuthType = "app_installation"

// Config contains the GitHub App installation settings needed by the GitHub integration.
type Config struct {
	AuthType       AuthType `yaml:"auth_type" env:"GITHUB_AUTH_TYPE"`
	AppID          int64    `yaml:"app_id" env:"GITHUB_APP_ID" validate:"required,gt=0"`
	InstallationID int64    `yaml:"installation_id" env:"GITHUB_INSTALLATION_ID" validate:"required,gt=0"`
	PrivateKeyPath string   `yaml:"private_key_path" env:"GITHUB_PRIVATE_KEY_PATH" validate:"required"`
	// APIURL overrides the GitHub REST API base URL. Leave empty to use the default.
	APIURL string    `yaml:"api_url" env:"GITHUB_API_URL"`
	MCP    MCPConfig `yaml:"mcp"`
}

type MCPConfig struct {
	Transport             mcp.Transport     `yaml:"transport" env:"GITHUB_MCP_TRANSPORT"`
	StdioCommand          string            `yaml:"stdio_command" env:"GITHUB_MCP_STDIO_COMMAND"`
	StdioArgs             []string          `yaml:"stdio_args"`
	StdioEnv              map[string]string `yaml:"stdio_env"`
	StreamableHTTPURL     string            `yaml:"streamable_http_url" env:"GITHUB_MCP_STREAMABLE_HTTP_URL"`
	StreamableHTTPHeaders map[string]string `yaml:"streamable_http_headers"`
	DisableStandaloneSSE  bool              `yaml:"disable_standalone_sse"`
	SSEURL                string            `yaml:"sse_url" env:"GITHUB_MCP_SSE_URL"`
	SSEHeaders            map[string]string `yaml:"sse_headers"`
	Toolsets              string            `yaml:"toolsets" env:"GITHUB_MCP_TOOLSETS"`
	Readonly              bool              `yaml:"readonly" env:"GITHUB_MCP_READONLY"`
}

func (c MCPConfig) Configured() bool {
	return c.Transport != "" || c.StdioCommand != "" || c.StreamableHTTPURL != "" || c.SSEURL != ""
}

func (c MCPConfig) config(token AccessToken) mcp.Config {
	transport := c.Transport
	if transport == "" {
		switch {
		case c.StreamableHTTPURL != "":
			transport = mcp.TransportStreamableHTTP
		case c.SSEURL != "":
			transport = mcp.TransportSSE
		default:
			transport = mcp.TransportStdio
		}
	}
	cfg := mcp.Config{Transport: transport}
	cfg.Stdio = mcp.StdioConfig{
		Command: c.StdioCommand,
		Args:    append([]string{}, c.StdioArgs...),
		Env:     cloneStringMap(c.StdioEnv),
	}
	if token.Value != "" {
		if cfg.Stdio.Env == nil {
			cfg.Stdio.Env = map[string]string{}
		}
		if _, exists := cfg.Stdio.Env["GITHUB_PERSONAL_ACCESS_TOKEN"]; !exists {
			cfg.Stdio.Env["GITHUB_PERSONAL_ACCESS_TOKEN"] = token.Value
		}
	}
	cfg.StreamableHTTP = mcp.StreamableHTTPConfig{
		URL:                  c.StreamableHTTPURL,
		Headers:              c.httpHeaders(token, c.StreamableHTTPHeaders),
		DisableStandaloneSSE: c.DisableStandaloneSSE,
	}
	cfg.SSE = mcp.SSEConfig{
		URL:     c.SSEURL,
		Headers: c.httpHeaders(token, c.SSEHeaders),
	}
	return cfg
}

func (c MCPConfig) httpHeaders(token AccessToken, configured map[string]string) map[string]string {
	headers := cloneStringMap(configured)
	if headers == nil {
		headers = map[string]string{}
	}
	if token.Value != "" {
		headers["Authorization"] = "Bearer " + token.Value
	}
	if c.Toolsets != "" {
		headers["X-MCP-Toolsets"] = c.Toolsets
	}
	if c.Readonly {
		headers["X-MCP-Readonly"] = "true"
	}
	return headers
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

// Configured reports whether any GitHub integration config was provided.
func (c Config) Configured() bool {
	return c.AuthType != "" || c.AppID != 0 || c.InstallationID != 0 || c.PrivateKeyPath != "" || c.APIURL != "" || c.MCP.Configured()
}

func apiURL(value string) (string, error) {
	if value == "" {
		return defaultAPIURL, nil
	}
	u, err := url.ParseRequestURI(value)
	if err != nil {
		return "", fmt.Errorf("invalid api_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid api_url scheme %q", u.Scheme)
	}
	return value, nil
}
