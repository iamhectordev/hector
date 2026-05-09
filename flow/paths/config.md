# Config

Typed application config loaded once at startup, with each component owning its own config struct.

## Principles
- Each component defines its own config type next to the component that consumes it.
- The app config composes component configs; it does not re-declare their fields.
- `cmd/<app>/main.go` loads typed config via `klee` before running commands.
- Commands read config from `context.Context` and wire dependencies from typed values.
- Config loading and config validation are separate steps.
- Consumers validate only the config required for the dependency they are about to construct.
- Do not read raw env vars in command actions when the value belongs in config.
- Provider or implementation selection belongs in config, not in ad hoc `if env != ""` branches.
- Use validation tags and a validator at the consumer boundary when that keeps rules local and readable.

## Outline
```go
// component-owned config
package llm

type Provider string

const (
    ProviderEcho   Provider = "echo"
    ProviderOpenAI Provider = "openai"
)

type OpenAIConfig struct {
    APIKey string `yaml:"api_key" env:"OPENAI_API_KEY"`
    Model  string `yaml:"model" env:"OPENAI_MODEL" default:"gpt-4o-mini"`
}

type Config struct {
    DefaultProvider Provider     `yaml:"default_provider" env:"LLM_DEFAULT_PROVIDER" default:"echo" validate:"oneof=echo openai"`
    OpenAI          OpenAIConfig `yaml:"openai"`
}

// app config
package main

type Config struct {
    LLM   llm.Config   `yaml:"llm"`
    Slack slack.Config `yaml:"slack"`
}

func main() {
    app := klee.New[Config]("hector", version, cli.Commands())
    _ = app.LoadConfig(klee.ConfigOptions[Config]{FlagArgs: os.Args})
    os.Exit(app.Run(ctx, os.Args))
}

func chatAction(ctx context.Context, _ *cli.Command) error {
    cfg := klee.Config[main.Config](ctx)

    completer, err := llm.New(cfg.LLM)
    if err != nil {
        return err
    }

    return runChat(completer)
}
```

## Example
- `pkg/llm` owns provider-selection config and can expose a factory like `llm.New(cfg, ...)`.
- `modules/slack` owns Slack credentials config and validates it when `serve` constructs the Slack module.
- `cmd/hector` defines the app config that composes them.
- `internal/cli/chat.go` and `internal/cli/serve.go` read typed config from context and choose which dependencies to wire.
- Validation happens when the command or factory is about to construct the dependency, not during config loading.
- Use env vars for provider selection when you want no YAML (for example `LLM_DEFAULT_PROVIDER=openai` with `OPENAI_API_KEY` set).
