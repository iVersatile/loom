#!/bin/sh
# migrate-loom-dev.sh — one-time cutover from `loom-loom-dev` to `loom-dev`
# Origin : T11/T13/T14 (PR #10) — rename + project mount + agent-home volume are
#          create-time docker state, so the old container must be replaced once.
# Reuse  : one-off for THIS rename; the pattern (old identity → teardown → build
#          → verify → re-auth) recurs on any container-identity change (T17).
# Runs on: the Mac host (docker + the new loom binary). NOT inside devenv or
#          loom-dev — removing the container you are sitting in strands you.
set -eu

OLD="${OLD_CONTAINER:-loom-loom-dev}"
NEW="${NEW_CONTAINER:-loom-dev}"
LOOM="${LOOM_BIN:-./bin/loom-darwin-arm64}"

say() { printf '\n== %s\n' "$*"; }

say "preflight"
command -v docker >/dev/null 2>&1 || { echo "ERROR: docker not found — run on the Mac host"; exit 1; }
[ -x "$LOOM" ] || command -v "$LOOM" >/dev/null 2>&1 || { echo "ERROR: loom binary not found at $LOOM (set LOOM_BIN; must include PR #10)"; exit 1; }
[ -f loom.yml ] || { echo "ERROR: run from the project root (no loom.yml here)"; exit 1; }
if [ -f /.dockerenv ]; then
  echo "ERROR: this looks like a container ($(whoami)@$(hostname)) — run on the Mac host"; exit 1
fi
# Stale-binary guard (learned 2026-06-10: first migration ran a pre-PR-#10 binary,
# which recreated the OLD identity with no mounts/volume and lost the creds).
# A post-#10 engine embeds the loom.managed label string; refuse anything older.
grep -aq 'loom.managed' "$LOOM" || {
  echo "ERROR: $LOOM predates the T11/T13/T14 engine (no loom.managed label support)."
  echo "Rebuild from current main first:  GOOS=darwin GOARCH=arm64 go build -o bin/loom-darwin-arm64 ./cmd/loom"
  exit 1
}

say "state before (the audit trail)"
docker ps -a --filter "name=$OLD" --filter "name=$NEW" || true
docker volume ls --filter "name=$NEW-claude" || true

if docker container inspect "$OLD" >/dev/null 2>&1; then
  echo "About to remove the OLD container '$OLD'."
  echo "Its in-container credentials die with it (writable home, pre-volume)."
  echo "Make sure no session is running inside it."
  if [ "${1:-}" != "--yes" ]; then
    printf "type 'yes' to remove %s: " "$OLD"
    read -r answer
    [ "$answer" = "yes" ] || { echo "aborted"; exit 1; }
  fi
  say "removing $OLD"
  docker rm -f "$OLD"
else
  say "old container $OLD already absent — continuing (re-runnable)"
fi

say "building $NEW via the new engine (mounts + volume + labels land at create)"
"$LOOM" build

say "verify"
docker ps --filter label=loom.managed=true
# The new container existing under the NEW name is the migration's definition of
# success — a hard failure, not a warning (a stale engine recreates the old name).
docker container inspect "$NEW" >/dev/null 2>&1 || {
  echo "ERROR: $NEW was not created — the build produced something else (stale binary?)"
  docker ps -a --filter label=loom.managed=true --filter "name=$OLD" || true
  exit 1
}
docker exec "$NEW" sh -lc 'claude --version' \
  || echo "WARN: claude not on PATH in $NEW (agent install failed?)"
docker exec "$NEW" sh -lc 'ls /workspace >/dev/null' \
  && echo "OK: project mounted at /workspace" \
  || echo "WARN: no /workspace mount in $NEW"
docker volume inspect "$NEW-claude" >/dev/null 2>&1 \
  && echo "OK: agent-home volume $NEW-claude present (creds will survive rebuilds)" \
  || echo "WARN: volume $NEW-claude missing"

say "final step — HUMAN: the one-time login (persists in the $NEW-claude volume)"
echo "  docker exec -it $NEW sh -lc claude"
echo "Complete the OAuth flow once; subsequent --force/teardown keep the credentials."
