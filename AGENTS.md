Use semantic commits with scope where relevant.
Do not code anything until the user confirms it.
All designs go through the user. Do not design anything alone.
Run mise run {lint,test} after each change.

Before building any slice:
1. Identify which golden paths apply (if any) — see flow/paths/
2. Outline files and signatures to add or change
3. Define how it will be validated (blackbox preferred)
4. Flag any missing golden paths — design them with the user before proceeding

Read more:
- Architecture decisions: docs/architecture.md
- Golden paths: flow/paths/

## Go API Design
- Use errors, not panic.
- Functions must not receive `nil`, `""`, or `0` to mean "no value" or "disabled".
- Optional behaviour: use the `With*` functional options pattern.
- Required dependencies: prefer an interface; never accept a concrete pointer that callers pass as `nil`.

## Go Testing
- Use the external test package when possible.
- Test the public API, not internals.
- Use `t.Context()` for context propagation.
- Use `testify/require` for assertions.
- Prefer table-driven tests when they help.
- Cover behavior well without too many repetitive tests.
