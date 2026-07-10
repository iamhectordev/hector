package slack

import (
	"github.com/doron-cohen/klee/secrets"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type Config struct {
	Enabled    bool              `yaml:"enabled" env:"SLACK_ENABLED"`
	AppToken   secrets.Secret    `yaml:"app_token" env:"SLACK_APP_TOKEN" secret:"slack_app_token" validate:"required_if=Enabled true"`
	BotToken   secrets.Secret    `yaml:"bot_token" env:"SLACK_BOT_TOKEN" secret:"slack_bot_token" validate:"required_if=Enabled true"`
	// APIURL overrides the Slack API base URL. Leave empty to use the default.
	APIURL     string            `yaml:"api_url" env:"SLACK_API_URL"`
	EventLog   EventLogConfig    `yaml:"event_log"`
	// AllowUsers is a list of Slack user IDs permitted to interact with Hector.
	// Empty means all users are allowed.
	AllowUsers []string          `yaml:"allow_users"`
}
