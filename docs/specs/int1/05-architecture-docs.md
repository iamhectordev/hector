# int1-05 — Make docs/architecture.md match reality

Design: `docs/designs/001-integration-facets.md`
Depends: int1-03, int1-04 (documents the end state)

## Goal

`docs/architecture.md` describes a `services/` layout that has never matched
this repo (`modules/`, root `internal/`, `pkg/`). Rewrite the Structure section
to document the actual layout including the new `integrations/` layer.

## Changes

- Replace the `services/` tree and prose with the real layout: `cmd/`,
  `internal/` (app wiring + shared infra only), `modules/` (core supervised
  modules: agent, tools executor, memory, email, tui), `pkg/` (public library
  surface: tools contract, supervisor, waffle, comms, llm, telem, …),
  `integrations/` (facet contracts + one package per vendor).
- Document the module/integration split in a short paragraph: module = runtime
  unit under `pkg/supervisor`; integration = capability unit exposing facets
  (tools / event source / surface); the generic host adapts one to the other;
  grouping into processes is a deployment decision, preserving the
  modular-monolith → split-services story.
- State the import discipline: integrations import only `pkg/...`; modules
  never import integrations except where explicitly flagged as debt (agent →
  slack events, see design doc "Known deferred issues").
- Update the DDD/interfaces section only where it names `services/`; keep its
  principles (consumer-owned ports, compile-time assertions) — they still hold.
- Do not touch Runtime, Configuration, or NFR sections except renaming
  `services` where it appears.
- Keep the document the same order of magnitude in length. No new sections
  beyond the above.

## Non-goals

- No code changes.
- No rewriting of docs/00-hector.md or docs/01-principles.md.

## Gate

`mise run lint` (markdown untouched by lint is fine — gate is a no-op guard
that nothing else changed). Reviewer (orchestrator) reads the diff against the
real tree.
