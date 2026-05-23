# Secret

A credential or sensitive value required by an integration.

## Principles
- Config should store secret references, paths, or environment bindings, not secret material when avoidable.
- Local development may use env vars or files outside the repository.
- Production should use a secret manager or encrypted credential store.
- Resolve secrets only at the point of use.
- Never log, render, return, persist in events, or place secrets in model context.
- Short-lived derived tokens should be cached only until expiry and not stored durably.

## Outline
```go
type Config struct {
    PrivateKeyPath string `yaml:"private_key_path" env:"APP_PRIVATE_KEY_PATH" validate:"required"`
}

func NewClient(cfg Config) (*Client, error) {
    key, err := os.ReadFile(cfg.PrivateKeyPath)
    if err != nil {
        return nil, fmt.Errorf("app: load private key: %w", err)
    }
    // Keep secret material inside the client/auth layer.
    _ = key
    return &Client{}, nil
}
```

## Example
- `modules/github` reads a GitHub App private key from a local path and caches only short-lived installation tokens in memory.
