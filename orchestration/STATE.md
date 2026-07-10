# Orchestration state — int1: Integration facets

Design: `docs/designs/001-integration-facets.md` · Orchestrator: `orchestration/orch.sh`
Workers: opencode `big-pickle` in `.worktrees/<issue>` on branch `int1/<issue>`.
Session IDs live in `.worktrees/logs/<issue>.ndjson` (`orch.sh session <issue>`).

| Issue | Spec | Depends | Status | Notes |
|---|---|---|---|---|
| int1-01 | docs/specs/int1/01-tools-to-pkg.md | — | integrated | gate pass attempt 1; pure renames, reviewed diff |
| int1-02 | docs/specs/int1/02-integration-contracts.md | int1-01 | integrated | gate pass attempt 1; contracts match design 1:1 |
| int1-03 | docs/specs/int1/03-github-integration.md | int1-02 | integrated | 2 rework rounds: stale-base rebase (false PASS claim), then clean --onto rebase merging slack+github wiring |
| int1-04 | docs/specs/int1/04-slack-integration.md | int1-02 | integrated | 1 rework round: worker stripped comments + mock logic during move; restored, re-verified |
| int1-05 | docs/specs/int1/05-architecture-docs.md | int1-03, int1-04 | running | docs only |

int1-03 and int1-04 can run in parallel once int1-02 is integrated (04's spec
covers the loop-merge if 03 hasn't landed).

Statuses: todo → running → review (worker done, awaiting verify/feedback) →
integrated | blocked.

## Log

- 2026-07-10: Design accepted (001-integration-facets), milestone sliced into 5
  specs, orchestrator ported from scales (mise gate, prompt-level boundaries —
  no guard plugin by decision).
