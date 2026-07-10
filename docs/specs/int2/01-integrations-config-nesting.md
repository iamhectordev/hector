# int2-01 — Nest integration config under `integrations:`

Design: `docs/designs/001-integration-facets.md` (config grouping; adaptation
layer stays deferred)
Golden paths: `flow/paths/integration.md`, `flow/paths/config.md`

## Goal

Give integrations a shared home in the config tree. Pure structural move:

```yaml
integrations:
  slack:
    enabled: true
    app_token: ...
  github:
    enabled: true
    app_id: ...
```

No field renames, no new fields, no behavior change beyond the yaml paths.

## Changes

### `internal/app/config.go`

- Add `IntegrationsConfig` struct with fields `Slack slack.Config
  \`yaml:"slack"\`` and `GitHub gh.Config \`yaml:"github"\``.
- `Config` gains `Integrations IntegrationsConfig \`yaml:"integrations"\``.
- Remove the top-level `GitHub` and `Slack` fields.
- All existing `env:` tags on integration config fields stay exactly as they
  are (env-based config keeps working unchanged).

### `internal/app/runtime.go`

- `buildIntegrations` reads `r.cfg.Integrations.Slack` and
  `r.cfg.Integrations.GitHub`.

### Callers and tests

- Grep the repo for `cfg.Slack`, `cfg.GitHub`, `Config{` literals with
  `Slack:`/`GitHub:` fields and update every reference (expect hits in
  `internal/app/*_test.go`, `internal/cli/*_test.go`).
- Grep docs and any committed example/config fixtures for top-level `slack:`
  or `github:` yaml keys and nest them under `integrations:`. Do NOT touch
  the untracked root `hector.yaml` (developer-local file).

### `flow/paths/integration.md`

Add one short paragraph under Principles: integration config lives under
`integrations.<name>` in app config; the sub-block names `tools`, `events`,
and `auth` are reserved for future shared conventions — vendor-specific
config must not use those names.

## Non-goals

- No adaptation fields (`include`, `pin`, `guidance`) — deferred until used.
- No moving `web_search` or `email` under `integrations:`.
- No changes inside `integrations/slack` or `integrations/github` packages
  (their Config structs are unchanged; only where they hang in app config).

## Proof

Paste the real output of:

- `grep -rn "cfg\.Slack\|cfg\.GitHub\|r\.cfg\.Slack\|r\.cfg\.GitHub" internal/ | grep -v Integrations` — must print nothing.
- `go test ./internal/app/... -count=1` last 5 lines.

## Gate

`mise run test && mise run lint`.
