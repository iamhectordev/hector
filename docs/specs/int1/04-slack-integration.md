# int1-04 — Slack as an integration

Design: `docs/designs/001-integration-facets.md`
Golden paths: `flow/paths/integration.md` (new version), `flow/paths/module.md`,
`flow/paths/waffle-event.md`, `flow/paths/config.md`
Depends: int1-02 (not on int1-03; if int1-03 landed first, extend its
`initIntegrations` loop instead of creating it)

## Goal

Fold `internal/slack` + `modules/slack` into `integrations/slack` implementing
the facet contracts; delete the hand-written Slack module. Add an explicit
`Enabled` gate and the serve-profile "at least one integration" rule. Apart from
that rule, behavior is preserved: same events with same data land on the bus.

## Changes

### `integrations/slack` (package `slack`)

Move the contents of `internal/slack` (events, parsing, enrichment, event log,
reply handler) and `modules/slack` (socket module, handlers, config) into one
package. Pure move: keep the `messageHandler` indirection, `allowUsers`
wrapper, `WithEventLogger` option, and event-log config exactly as they are.

```go
// Integration implements integrations.Integration, integrations.Initializer,
// integrations.EventSource, integrations.Surface, io.Closer.
func New(bus *waffle.EventBus, cfg *Config, opts ...Option) (*Integration, error)
```

- `Name()` → `"slack"`.
- `Init(ctx)` — today's `Module.Init`: auth test, build API + socketmode clients.
- `Run(ctx)` — today's `Module.Start`/`run`: socket loop + event loop, blocks.
- `ReplyHandler()` — today's `NewReplyHandler()`; MUST keep the lazy client
  resolution (the reply handler is constructed at wiring time, before `Init`
  builds the API client).
- `Close()` — today's `Module.Stop`: closes the event logger.

Event definitions (`MessageReceived`, `MessageUpdated`, data types) keep their
event names and payloads byte-identical. `modules/agent` updates its imports
from `internal/slack` to `integrations/slack` — mechanical, in its own commit
(the agent→slack coupling is a known deferred issue; do not refactor it).

Check remaining importers of `internal/slack` (e.g. mocks, tests, config
loading) and update them. Delete `internal/slack` and `modules/slack` when
empty.

Note: `integrations/slack` imports only `pkg/...` + the slack SDK after the
move. If anything in `internal/slack` pulls other `internal/` packages, stop
and flag it rather than widening the move.

### Config

Add `Enabled bool` yaml:"enabled"` to slack `Config`. Token fields gain
`validate:"required_if=Enabled true"`-style enforcement consistent with the
github config (validation must not fire when disabled). Existing deployments:
this is a breaking config change — `serve` users must now set
`slack.enabled: true`; note it in the commit message body.

### Runtime wiring (`internal/app/runtime.go`)

- Remove the Slack branch from `initSurfaces`; `ProfileServe` no longer
  hard-creates Slack. `ProfileChat` keeps TUI exactly as-is.
- `initIntegrations` gains Slack (gated on `cfg.Slack.Enabled`). The wiring loop
  now also handles the `Surface` facet: collect `ReplyHandler()`s and pass them
  to `comms.NewReplyRouter`.
- New rule: under `ProfileServe`, if zero integrations are enabled, `init`
  returns an error: `"serve requires at least one enabled integration"`.

## Non-goals

- No changes to Slack event parsing, enrichment, allowlist, or event-log
  behavior.
- No fix to the agent↔slack event coupling.
- No trimming of options or config fields.

## Gate

`mise run test && mise run lint`. The existing `modules/slack`
`integration_test.go` moves and passes with only package/import adjustments —
its assertions (which events land on the bus, with what data) are unchanged.
Add runtime-level tests: serve + slack enabled wires the reply router and host;
serve with nothing enabled errors with the message above; chat profile is
unaffected by the rule.
