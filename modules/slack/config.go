package slack

import (
	"github.com/go-playground/validator/v10"

	islack "github.com/iamhectordev/hector/internal/slack"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type Config struct {
	AppToken string `yaml:"app_token" env:"SLACK_APP_TOKEN" validate:"required"`
	BotToken string `yaml:"bot_token" env:"SLACK_BOT_TOKEN" validate:"required"`
	// APIURL overrides the Slack API base URL. Leave empty to use the default.
	APIURL   string                `yaml:"api_url" env:"SLACK_API_URL"`
	EventLog islack.EventLogConfig `yaml:"event_log"`
	// AllowUsers is a list of Slack user IDs permitted to interact with Hector.
	// Empty means all users are allowed.
	AllowUsers []string `yaml:"allow_users"`
}
