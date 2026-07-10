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
| int1-05 | docs/specs/int1/05-architecture-docs.md | int1-03, int1-04 | integrated | 1 rework round: four tree annotations were factually wrong; corrected and re-verified |

**Milestone complete** — all five slices integrated on 2026-07-10.

int1-03 and int1-04 can run in parallel once int1-02 is integrated (04's spec
covers the loop-merge if 03 hasn't landed).

Statuses: todo → running → review (worker done, awaiting verify/feedback) →
integrated | blocked.

## Conventions (post-int1 retro, 2026-07-10)

- Specs may include a `## Proof` section: commands whose pasted real output is
  part of the worker's exit criteria. Move slices should demand
  `git diff --find-renames --stat main -- <moved paths>` showing 100% rename
  similarity for every file the spec doesn't explicitly change.
- The worker prompt demands repo-state evidence before the RESULT line
  (clean porcelain, main ancestry check) and a self-diff review against the
  spec's Non-goals. Rationale: every int1 rework traced to an unproven claim —
  tidied-away comments during moves, a stale-base rebase reported as clean,
  docs annotations written from memory instead of from the code.
- Per-slice extra checks belong in the gate (spawn's 4th arg), not in prose.
  A reusable move-fidelity gate script is deferred until a move slice needs it.

## Log

- 2026-07-10: Design accepted (001-integration-facets), milestone sliced into 5
  specs, orchestrator ported from scales (mise gate, prompt-level boundaries —
  no guard plugin by decision).
