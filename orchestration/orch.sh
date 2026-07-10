#!/usr/bin/env bash
# Orchestrator for opencode workers building hector milestones.
# Usage:
#   orch.sh spawn  <issue-id> <spec-path> [base-branch] [gate-cmd]  # worktree+branch, run worker, then gate loop
#   orch.sh resume <issue-id> "<feedback>"                # continue the worker's session, then gate loop
#   orch.sh gate   <issue-id>                             # run the issue's gate loop (verify + auto-feedback, max 3)
#   orch.sh verify <issue-id>                             # mise run test && mise run lint in the worktree (exit code only)
#   orch.sh integrate <issue-id>                          # rebase on main, verify, merge --no-ff, clean up
#   orch.sh status                                        # one line per worker: last event + verify hint
#   orch.sh session <issue-id>                            # print the opencode session id
#
# Gate loop: after a worker finishes, its gate command (default
# "mise run test && mise run lint", override per issue via spawn's 4th arg) runs
# in the worktree. On failure the real output is sent back into the worker's
# session; after GATE_MAX_ATTEMPTS failures the loop gives up and flags the
# issue for the orchestrator. Every attempt is logged with output to
# .worktrees/logs/<issue>.gate.log.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WT_DIR="$ROOT/.worktrees"
LOG_DIR="$WT_DIR/logs"
MODEL="${ORCH_MODEL:-opencode/big-pickle}"
BRANCH_PREFIX="${ORCH_BRANCH_PREFIX:-int1}"
DEFAULT_GATE="mise run test && mise run lint"

GATE_MAX_ATTEMPTS=3

wt() { echo "$WT_DIR/$1"; }
branch_of() { echo "$BRANCH_PREFIX/$1"; }
log_of() { echo "$LOG_DIR/$1.ndjson"; }
gate_cmd_of() {
  if [ -f "$LOG_DIR/$1.gate" ]; then cat "$LOG_DIR/$1.gate"; else echo "$DEFAULT_GATE"; fi
}

# Run the issue's gate once; append decision + output tail to the gate log.
run_gate() {
  local issue="$1" attempt="$2" dir gate out rc=0
  dir="$(wt "$issue")"; gate="$(gate_cmd_of "$issue")"
  out="$LOG_DIR/$issue.gate-out.tmp"
  (cd "$dir" && bash -o pipefail -c "$gate") >"$out" 2>&1 || rc=$?
  {
    echo "[$(date '+%F %T')] $issue gate attempt $attempt exit=$rc cmd: $gate"
    tail -n 30 "$out"
    echo "--------"
  } >>"$LOG_DIR/$issue.gate.log"
  return $rc
}

# Verify-retry loop: gate fail -> feed real output back to the author session.
gate_loop() {
  local issue="$1" attempt sid dir
  dir="$(wt "$issue")"
  for attempt in $(seq 1 "$GATE_MAX_ATTEMPTS"); do
    if run_gate "$issue" "$attempt"; then
      echo "[orch] $issue: GATE PASS (attempt $attempt)"
      return 0
    fi
    if [ "$attempt" -lt "$GATE_MAX_ATTEMPTS" ]; then
      echo "[orch] $issue: GATE FAIL (attempt $attempt) — feeding output back to worker"
      sid="$(session_of "$issue")"
      (cd "$dir" && opencode run -s "$sid" "The verification gate failed; your work is not done. Gate command: $(gate_cmd_of "$issue")

Last 30 lines of real output:
$(tail -n 30 "$LOG_DIR/$issue.gate-out.tmp")

Fix the failures (stay within your spec's scope), commit, leave the tree clean. End with 'RESULT: PASS <summary>' or 'RESULT: FAIL <blocker>'." \
        --model "$MODEL" --format json >>"$(log_of "$issue")" 2>>"$LOG_DIR/$issue.err") || true
    fi
  done
  echo "[orch] $issue: GATE FAIL after $GATE_MAX_ATTEMPTS attempts — orchestrator attention needed"
  return 1
}

session_of() {
  local log; log="$(log_of "$1")"
  [ -f "$log" ] || { echo "no log for $1" >&2; return 1; }
  awk 'match($0, /"sessionID":"[^"]*"/) { s = substr($0, RSTART+13, RLENGTH-14); print s; exit }' "$log"
}

worker_prompt() {
  local issue="$1" spec="$2" branch="$3"
  cat <<EOF
You are an autonomous engineer on the hector Go codebase, in a git worktree on branch ${branch}.

Task: implement the spec at ${spec} exactly.

Boundaries (you are trusted, not sandboxed — honor these):
- Work only inside this worktree. Never edit files outside it.
- Never push, never touch git remotes, never use the gh CLI.
- Never run 'git -C' pointing outside this worktree, 'git worktree', or delete branches. The orchestrator owns integration.

Process:
1. Read ${spec} fully. Read AGENTS.md. Read the design doc and every golden path the spec names under flow/paths/.
2. Explore the packages the spec names before writing code; match existing patterns.
3. Implement the spec. No scope creep: only what the spec requires, only in the files/packages it scopes. If the spec says to stop and flag something, stop and report it in your final message instead of improvising.
4. Add every test the spec lists. Run 'mise run test' and 'mise run lint' and fix failures until both pass cleanly.
5. Commit all work on the current branch in small semantic commits (scope where relevant). Do not push. Leave the working tree clean.
6. Evidence, not verdicts: in your final message, paste the last 5 lines of the real output of each verification command you ran (no cached results — use -count=1 if you invoke go test directly). A PASS claim without pasted output is a failure.
7. End your final message with exactly one line: 'RESULT: PASS <one-line summary>' or 'RESULT: FAIL <what blocked you>'.
EOF
}

cmd_spawn() {
  local issue="$1" spec="$2" base="${3:-main}" gate="${4:-}" branch dir
  branch="$(branch_of "$issue")"; dir="$(wt "$issue")"
  mkdir -p "$LOG_DIR"
  [ -f "$ROOT/$spec" ] || { echo "spec not found: $spec" >&2; exit 1; }
  [ -n "$gate" ] && printf '%s\n' "$gate" >"$LOG_DIR/$issue.gate"
  if [ ! -d "$dir" ]; then
    git -C "$ROOT" worktree add "$dir" -b "$branch" "$base" >/dev/null
  fi
  echo "[orch] $issue: worker starting on $branch (model $MODEL)"
  cd "$dir"
  opencode run "$(worker_prompt "$issue" "$spec" "$branch")" \
    --model "$MODEL" --format json >"$(log_of "$issue")" 2>"$LOG_DIR/$issue.err"
  echo "[orch] $issue: worker exited $?"
  tail -c 2000 "$(log_of "$issue")" | grep -o 'RESULT: \(PASS\|FAIL\)[^"]*' | tail -1 || echo "[orch] $issue: no RESULT line found"
  gate_loop "$issue"
}

cmd_resume() {
  local issue="$1" feedback="$2" sid dir
  sid="$(session_of "$issue")"; dir="$(wt "$issue")"
  cd "$dir"
  opencode run -s "$sid" "$feedback

End your final message with exactly one line: 'RESULT: PASS <summary>' or 'RESULT: FAIL <blocker>'." \
    --model "$MODEL" --format json >>"$(log_of "$issue")" 2>>"$LOG_DIR/$issue.err"
  echo "[orch] $issue: resume exited $?"
  tail -c 2000 "$(log_of "$issue")" | grep -o 'RESULT: \(PASS\|FAIL\)[^"]*' | tail -1 || true
  gate_loop "$issue"
}

cmd_verify() (
  local issue="$1" dir; dir="$(wt "$issue")"
  cd "$dir"
  if mise run test >/dev/null 2>&1 && mise run lint >/dev/null 2>&1; then
    echo "[orch] $issue: VERIFY PASS"
  else
    echo "[orch] $issue: VERIFY FAIL (rerun 'mise run test' / 'mise run lint' in $dir for details)"
    return 1
  fi
)

cmd_integrate() {
  local issue="$1" branch dir
  branch="$(branch_of "$issue")"; dir="$(wt "$issue")"
  [ -z "$(git -C "$dir" status --porcelain)" ] || { echo "[orch] $issue: worktree dirty, refusing" >&2; exit 1; }
  if ! git -C "$dir" rebase main >/dev/null 2>&1; then
    git -C "$dir" rebase --abort
    echo "[orch] $issue: REBASE CONFLICT — resume the author session to resolve" >&2
    exit 2
  fi
  cmd_verify "$issue"
  git -C "$ROOT" merge --no-ff "$branch" -m "merge: $branch

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
  git -C "$ROOT" worktree remove "$dir"
  git -C "$ROOT" branch -d "$branch"
  echo "[orch] $issue: INTEGRATED into main"
}

cmd_status() {
  local log issue sid alive age last_ts now
  now="$(date +%s)"
  for log in "$LOG_DIR"/*.ndjson; do
    [ -e "$log" ] || { echo "no workers yet"; return; }
    issue="$(basename "$log" .ndjson)"
    sid="$(session_of "$issue" 2>/dev/null || true)"
    # Liveness heuristic: a worker streams events constantly while alive, so
    # recent output is the reliable signal (argv-based pgrep misses first runs).
    alive="idle"
    pgrep -f "opencode run.*$sid" >/dev/null 2>&1 && alive="RUNNING"
    last_ts="$(awk 'match($0, /"timestamp":[0-9]+/) { t = substr($0, RSTART+12, RLENGTH-12) } END { print t }' "$log")"
    age="?"
    if [ -n "$last_ts" ]; then
      age="$(( now - last_ts / 1000 ))s ago"
      [ $(( now - last_ts / 1000 )) -lt 120 ] && alive="RUNNING"
    fi
    printf '%s [%s, last event %s]: ' "$issue" "$alive" "$age"
    tail -c 2000 "$log" | grep -o 'RESULT: \(PASS\|FAIL\)[^"]*' | tail -1 ||
      { tail -1 "$log" | grep -o '"type":"[^"]*"' | head -1; } || echo "(no events)"
  done
}

case "${1:-}" in
  spawn)     shift; cmd_spawn "$@" ;;
  resume)    shift; cmd_resume "$@" ;;
  gate)      shift; gate_loop "$@" ;;
  verify)    shift; cmd_verify "$@" ;;
  integrate) shift; cmd_integrate "$@" ;;
  status)    shift || true; cmd_status ;;
  session)   shift; session_of "$@" ;;
  *) grep '^#' "$0" | head -10; exit 1 ;;
esac
