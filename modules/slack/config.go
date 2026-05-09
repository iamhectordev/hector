package slack

type Config struct {
	AppToken string `yaml:"app_token" env:"SLACK_APP_TOKEN" validate:"required"`
	BotToken string `yaml:"bot_token" env:"SLACK_BOT_TOKEN" validate:"required"`
}
