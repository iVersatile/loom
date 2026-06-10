#!/bin/sh
# verify-loom-dev.sh — claims check for a fresh loom-dev session (predict → verify)
# Origin : the T12 cutover; same predict-first pattern as
#          .scratch/session-start-verification.md. PRESERVE claims pin what the
#          engine guarantees (T8/T11/T13/T14); GAP claims pin what is knowingly
#          missing until T16 — a GAP turning into a PASS is a surprise worth
#          reporting, not silently enjoying.
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

for t in go jq rg gitleaks git; do
  command -v "$t" >/dev/null 2>&1 && ok "tool present: $t" || bad "tool missing: $t"
done

[ -x "$HOME/.claude/statusline.sh" ] \
  && ok "statusline materialized + executable" \
  || bad "statusline missing — materialize regression"

hooks_path=$(git -C /workspace/loom config core.hooksPath 2>/dev/null || true)
[ "$hooks_path" = ".githooks" ] \
  && ok "repo gate hooks active (core.hooksPath=.githooks travels with the tree)" \
  || bad "gate hooks NOT active (core.hooksPath='$hooks_path') — run: git config core.hooksPath .githooks"

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

[ -d "$HOME/.claude/hooks" ] \
  && spr "harness hooks dir present" \
  || gap "no harness hooks — no session snapshot, no guard-bash"

[ -d "$HOME/.claude/projects" ] \
  && spr "memory/projects dir present" \
  || gap "no memory — repo docs (OPEN-THREADS, handoff) are the only durable memory"

[ -d "$HOME/.claude/skills" ] \
  && spr "skills dir present" \
  || gap "no skills/plugins"

grep -q '"hooks"\|"permissions"' "$HOME/.claude/settings.json" 2>/dev/null \
  && spr "settings.json has hooks/permissions" \
  || gap "settings.json is statusLine-only — expect extra permission prompts"

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
