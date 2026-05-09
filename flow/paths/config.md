# Config

Typed application config loaded once at startup, with each component owning its own config struct.

## Principles
- Each component defines its own config type next to the component that consumes it.
- The app config composes component configs; it does not re-declare their fields.
- `cmd/<app>/main.go` loads typed config via `klee` before running commands.
- Commands read config from `context.Context` and wire dependencies from typed values.
- Config loading and config validation are separate steps.
- Validation tags belong on config struct fields, not on anonymous or throwaway structs.
- A package-level `validate` instance lives next to the config it validates — not inside functions.
- The constructor of a component validates its own config slice; `NewFoo(cfg) (*Foo, error)`.
- `validate.Struct(cfg.SubConfig)` at the consumer boundary; no `dive` needed for nested structs (only for slices and maps).
- Do not read raw env vars in command actions when the value belongs in config.
- Provider or implementation selection belongs in config, not in ad hoc `if env != ""` branches.

## Outline
```go
// component config — tags on fields, package-level validator
package slack

var validate = validator.New(validator.WithRequiredStructEnabled())

type Config struct {
    AppToken string `yaml:"app_token" env:"SLACK_APP_TOKEN" validate:"required"`
    BotToken string `yaml:"bot_token" env:"SLACK_BOT_TOKEN" validate:"required"`
}

// constructor validates its own config slice and returns an error
func NewModule(bus *EventBus, cfg Config, opts ...Option) (*Module, error) {
    if err := validate.Struct(cfg); err != nil {
        return nil, fmt.Errorf("slack: invalid config: %w", err)
    }
    // ...
}

// factory validates the sub-config it selects
package llm

var validate = validator.New(validator.WithRequiredStructEnabled())

type OpenAIConfig struct {
    APIKey string `yaml:"api_key" env:"OPENAI_API_KEY" validate:"required"`
    Model  string `yaml:"model" env:"OPENAI_MODEL" default:"gpt-4o-mini"`
}

func New(cfg Config, opts ...Option) (agent.Completer, error) {
    switch provider {
    case ProviderOpenAI:
        if err := validate.Struct(cfg.OpenAI); err != nil {
            return nil, fmt.Errorf("llm: openai config: %w", err)
        }
        return openai.New(cfg.OpenAI.APIKey, cfg.OpenAI.Model), nil
    }
}

// command just wires — no validation of its own
func serveAction(ctx context.Context, _ *cli.Command) error {
    cfg := klee.Config[main.Config](ctx)

    completer, err := llm.New(cfg.LLM)       // validates openai sub-config
    if err != nil {
        return err
    }
    slackModule, err := slack.NewModule(bus, cfg.Slack)  // validates slack config
    if err != nil {
        return err
    }
    // wire supervisor ...
}
```

## Example
- `pkg/llm` and `modules/slack` each own their config, define a package-level `validate`, and expose constructors that return errors.
- `cmd/hector` defines the app config that composes them.
- `internal/cli/serve.go` just wires — it does not validate config itself.
- Use env vars for provider selection when you want no YAML (for example `LLM_DEFAULT_PROVIDER=openai` with `OPENAI_API_KEY` set).
