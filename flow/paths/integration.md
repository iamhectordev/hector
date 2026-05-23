# Integration

A module that connects Hector to an external application or service.

## Principles
- The integration owns the config it needs to initialize and operate.
- Config loading and config validation stay separate; constructors validate the config they consume.
- Required dependencies are explicit fields or interfaces, not nil pointers.
- Secrets appear in config as references or local dev paths, not inline values.
- Clients are initialized from validated config and expose a small public API.
- Integrations provide a cheap verification path for credentials and connectivity.
- Tools, listeners, and renderers are added after the client boundary is proven.

## Outline
```go
type Config struct {
    APIURL string `yaml:"api_url" env:"APP_API_URL" validate:"omitempty,url"`
}

type Client struct {
    apiURL string
}

func NewClient(cfg Config, opts ...Option) (*Client, error) {
    if err := validate.Struct(cfg); err != nil {
        return nil, fmt.Errorf("app: invalid config: %w", err)
    }
    return &Client{apiURL: cfg.APIURL}, nil
}

func (c *Client) Verify(ctx context.Context) error {
    // Make the cheapest request that proves auth and connectivity.
    return nil
}
```

## Example
- `modules/github` owns GitHub App installation config, validates it in `NewClient`, initializes a REST client, and verifies access with one read-only request.
