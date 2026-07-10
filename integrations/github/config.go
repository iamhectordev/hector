package github

import (
	"fmt"
	"net/url"

	"github.com/go-playground/validator/v10"
)

const defaultAPIURL = "https://api.github.com"

var validate = validator.New(validator.WithRequiredStructEnabled())

type AuthType string

const AuthTypeAppInstallation AuthType = "app_installation"

// Config contains the GitHub App installation settings needed by the GitHub integration.
type Config struct {
	Enabled        bool     `yaml:"enabled"`
	AuthType       AuthType `yaml:"auth_type" env:"GITHUB_AUTH_TYPE"`
	AppID          int64    `yaml:"app_id" env:"GITHUB_APP_ID" validate:"required_if=Enabled true,gt=0"`
	InstallationID int64    `yaml:"installation_id" env:"GITHUB_INSTALLATION_ID" validate:"required_if=Enabled true,gt=0"`
	PrivateKeyPath string   `yaml:"private_key_path" env:"GITHUB_PRIVATE_KEY_PATH" validate:"required_if=Enabled true"`
	// APIURL overrides the GitHub REST API base URL. Leave empty to use the default.
	APIURL string    `yaml:"api_url" env:"GITHUB_API_URL"`
	MCP    MCPConfig `yaml:"mcp"`
}

type MCPConfig struct {
	Transport             string            `yaml:"transport" env:"GITHUB_MCP_TRANSPORT"`
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
