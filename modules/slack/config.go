package slack

import "github.com/go-playground/validator/v10"

var validate = validator.New(validator.WithRequiredStructEnabled())

type Config struct {
	AppToken string `yaml:"app_token" env:"SLACK_APP_TOKEN" validate:"required"`
	BotToken string `yaml:"bot_token" env:"SLACK_BOT_TOKEN" validate:"required"`
	// APIURL overrides the Slack API base URL. Leave empty to use the default.
	APIURL string `yaml:"api_url" env:"SLACK_API_URL"`
}
