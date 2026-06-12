#!/bin/sh
# verify-loom-dev.sh — claims check for a fresh loom-dev session (predict → verify)
# Origin : the T12 cutover; same predict-first pattern as
#          .scratch/session-start-verification.md. PRESERVE claims pin what the
#          engine guarantees (T8/T11/T13/T14); GAP claims pin what is knowingly
#          missing until T16 — a GAP turning into a PASS is a surprise worth
#          reporting, not silently enjoying. Gate-dependency claims added after
#          T19 (golangci-lint undeclared and undeclarable, found out-of-band):
#          probing only the playbook-declared tool list is structurally blind
#          to an undeclared dep, so the gate's own binaries are asserted too,
#          resolved exactly the way the Makefile resolves them.
# Reuse  : recurring — run at every loom-dev session start and after every
#          rebuild. Verb-candidate: each stable PRESERVE claim promotes to a
#          `loom doctor` check (SPEC-verbs doctor: tools present, hooks
#          executable, lock consistent, guardrails active) + an FR; the GAP
#          list shrinks as T16 lands and dies with it (T17 lifecycle).
# Runs on: INSIDE loom-dev (no docker needed; host-side checks like labels and
#          volume existence belong to the migration script / future doctor).
set -u

pass=0; fail=0
ok()  { pass=$((pass+1)); printf 'PASS  %s\n' "$*"; }
bad() { fail=$((fail+1)); printf 'FAIL  %s\n' "$*"; }
gap() { printf 'GAP   %s (expected until T16)\n' "$*"; }
spr() { printf 'SURPRISE  %s — revisit the model, see T16\n' "$*"; }

echo "== PRESERVE claims — engine guarantees, any FAIL is a regression =="

[ -f /.dockerenv ] && [ "$(whoami)" = "root" ] \
  && ok "inside a container as root (loom-dev, pre-T10)" \
  || bad "not root-in-container — are you in the right environment?"

[ -f /workspace/loom/loom.yml ] \
  && ok "project mounted at /workspace/loom (T13)" \
  || bad "no repo mount — T13 regression or stale container"

[ -f "$HOME/.claude/.credentials.json" ] \
  && ok "credentials present — agent-home volume survived (T14)" \
  || bad "no credentials — T14 volume failed; re-login required"

command -v claude >/dev/null 2>&1 \
  && ok "claude on PATH: $(command -v claude) (T8)" \
  || bad "claude not on PATH — T8 regression (check ~/.local/bin in .profile/.bashrc)"

for t in go jq rg gitleaks git make golangci-lint; do
  command -v "$t" >/dev/null 2>&1 && ok "tool present: $t" || bad "tool missing: $t"
done

# Gate dependencies, resolved the way the Makefile resolves them (T19): every
# binary `make gate` hard-requires must exist, whether or not the playbook
# declared it — this is the claim that would have caught golangci-lint.
gobin="$(go env GOPATH 2>/dev/null)/bin"
golangci="$(command -v golangci-lint 2>/dev/null || echo "$gobin/golangci-lint")"
[ -x "$golangci" ] \
  && ok "gate dep resolvable: golangci-lint ($golangci)" \
  || bad "gate dep missing: golangci-lint — make gate will hard-fail lint (T19)"
for t in make gofmt go gitleaks; do
  command -v "$t" >/dev/null 2>&1 \
    && ok "gate dep resolvable: $t" \
    || bad "gate dep missing: $t — make gate cannot run (T19)"
done

[ -x "$HOME/.claude/statusline.sh" ] \
  && ok "statusline materialized + executable" \
  || bad "statusline missing — materialize regression"

hooks_path=$(git -C /workspace/loom config core.hooksPath 2>/dev/null || true)
[ "$hooks_path" = ".githooks" ] \
  && ok "repo gate hooks active (core.hooksPath=.githooks travels with the tree)" \
  || bad "gate hooks NOT active (core.hooksPath='$hooks_path') — run: git config core.hooksPath .githooks"

# T16 PR 2 flips: the base playbook declares harness: (settings + guard-bash),
# so these stopped being losses and became guarantees. A FAIL right after the
# merge usually means the container wasn't rebuilt yet (stale-binary rule).
[ -x "$HOME/.claude/hooks/guard-bash" ] \
  && ok "harness hook materialized + executable: guard-bash (T16 PR 2)" \
  || bad "guard-bash missing from ~/.claude/hooks — harness wire-up regression (or rebuild pending)"

grep -q '"hooks"' "$HOME/.claude/settings.json" 2>/dev/null \
  && grep -q '"permissions"' "$HOME/.claude/settings.json" 2>/dev/null \
  && ok "settings.json declares hooks + permissions (T16 PR 2)" \
  || bad "settings.json lacks hooks/permissions — harness wire-up regression (or rebuild pending)"

# The lock is container-pinned (T5): the container should agree with it.
lock_go=$(sed -n '/^  go:/,/source:/p' /workspace/loom/loom.lock 2>/dev/null | sed -n 's/ *resolved: //p')
have_go=$(go version 2>/dev/null)
if [ -n "$lock_go" ] && [ "$lock_go" = "$have_go" ]; then
  ok "go matches loom.lock pin: $have_go (T5)"
else
  bad "go vs lock mismatch: lock='$lock_go' have='$have_go' — rebuild or re-pin"
fi

echo
echo "== EXPECTED LOSSES — known T16 gaps; a PASS here is a SURPRISE =="

[ -x "$HOME/.claude/hooks/session-snapshot" ] \
  && spr "session-snapshot hook present" \
  || gap "no session-snapshot hook — content design parked (judgment-trial C4)"

[ -d "$HOME/.claude/projects" ] \
  && spr "memory/projects dir present" \
  || gap "no memory — repo docs (OPEN-THREADS, handoff) are the only durable memory"

[ -d "$HOME/.claude/skills" ] \
  && spr "skills dir present" \
  || gap "no skills/plugins"

[ -n "$(git config --global user.email 2>/dev/null)" ] \
  && spr "global git identity set" \
  || gap "no git identity — before committing: git config --global user.name/user.email"

echo
echo "result: $pass pass / $fail fail"
if [ "$fail" -gt 0 ]; then
  echo "REGRESSION(s) above — investigate before doing real work."
  exit 1
fi
echo "loom-dev claims hold; gaps are the known T16 set."
exit 0
