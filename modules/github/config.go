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
	Command string            `yaml:"command" env:"GITHUB_MCP_COMMAND"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

func (c MCPConfig) Configured() bool {
	return c.Command != ""
}

func (c MCPConfig) config(token AccessToken) mcp.Config {
	env := make(map[string]string, len(c.Env)+1)
	for key, value := range c.Env {
		env[key] = value
	}
	if token.Value != "" {
		if _, exists := env["GITHUB_PERSONAL_ACCESS_TOKEN"]; !exists {
			env["GITHUB_PERSONAL_ACCESS_TOKEN"] = token.Value
		}
	}
	return mcp.Config{Command: c.Command, Args: c.Args, Env: env}
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
