#!/usr/bin/env bash
# probe-apikeyhelper.sh — T15 / ADR-0027 live spike. HOST-run (docker actuation = human).
#
# Confirms the two ADR-0027 unknowns by isolating apiKeyHelper as the ONLY cred path:
#   UNKNOWN 1a  headless `claude -p`  (automated below)
#   UNKNOWN 1b  interactive TUI       (one manual command, you observe)
#   UNKNOWN 2   billing/identity      (you supply the key type; PASS confirms it)
#
# SAFETY: the key is read from your env, piped in via STDIN (never `-e`, so it never
# lands in Config.Env / `docker inspect`), written 0600 inside, cat by the helper, and
# the container is force-removed on exit. The key is never echoed or logged.
#
# USAGE:
#   export PROBE_ANTHROPIC_KEY='sk-ant-api03-...'        # an Anthropic API key (x-api-key path)
#   IMAGE=loom-dev:latest bash probe-apikeyhelper.sh     # any image that has `claude`
#   # if the image lacks claude:  INSTALL_CLAUDE=1 IMAGE=debian:bookworm-slim bash ...
set -euo pipefail

: "${PROBE_ANTHROPIC_KEY:?export PROBE_ANTHROPIC_KEY with an Anthropic API key first}"
IMAGE="${IMAGE:-debian:bookworm-slim}"
INSTALL_CLAUDE="${INSTALL_CLAUDE:-0}"
CTR="t15-apikeyhelper-probe"

cleanup() { docker rm -f "$CTR" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker rm -f "$CTR" >/dev/null 2>&1 || true
docker run -d --name "$CTR" --entrypoint sleep "$IMAGE" infinity >/dev/null

if [ "$INSTALL_CLAUDE" = "1" ]; then
  docker exec "$CTR" sh -c 'command -v curl >/dev/null || (apt-get update && apt-get install -y curl ca-certificates >/dev/null)'
  docker exec "$CTR" sh -c 'command -v claude >/dev/null || (curl -fsSL https://claude.ai/install.sh | bash) >/dev/null 2>&1' || true
fi
docker exec "$CTR" sh -c 'command -v claude >/dev/null' \
  || { echo "FAIL: \`claude\` not found in image $IMAGE — set INSTALL_CLAUDE=1"; exit 1; }

HOME_IN="$(docker exec "$CTR" sh -c 'echo "$HOME"')"

# Wire the key (via STDIN, as the container user → correct ownership, no Config.Env leak)
# and the apiKeyHelper. In production this command is `op read op://...` etc.; the probe
# just cats the file — the point is whether claude HONORS apiKeyHelper, not the store.
printf '%s' "$PROBE_ANTHROPIC_KEY" | docker exec -i "$CTR" sh -c \
  'mkdir -p "$HOME/.probe" "$HOME/.claude" && cat > "$HOME/.probe/key" && chmod 600 "$HOME/.probe/key"'
docker exec "$CTR" sh -c \
  'printf "{\"apiKeyHelper\": \"cat %s/.probe/key\"}\n" "$HOME" > "$HOME/.claude/settings.json"
   rm -f "$HOME/.claude/.credentials.json"'   # ensure NO OAuth creds — apiKeyHelper must stand alone

echo "=== UNKNOWN 1a — headless: claude -p via apiKeyHelper (no env key, no OAuth) ==="
if docker exec -e ANTHROPIC_API_KEY= -e CLAUDE_CODE_OAUTH_TOKEN= "$CTR" \
     claude -p "Reply with exactly: PROBE_OK" >/tmp/probe.out 2>/tmp/probe.err; then
  if grep -q "PROBE_OK" /tmp/probe.out; then
    echo "  PASS — apiKeyHelper authenticated headless \`claude -p\`."
  else
    echo "  UNCLEAR — ran but no PROBE_OK. output:"; sed 's/^/    /' /tmp/probe.out | head
  fi
else
  echo "  FAIL — headless did not authenticate. stderr:"; sed 's/^/    /' /tmp/probe.err | head
fi

cat <<EOF

=== UNKNOWN 1b — interactive TUI (manual: run it, then OBSERVE) ===
  docker exec -it -e ANTHROPIC_API_KEY= -e CLAUDE_CODE_OAUTH_TOKEN= $CTR claude

  PASS = drops into the TUI authenticated / answers a prompt (apiKeyHelper drives the TUI).
  FAIL = it asks you to log in (apiKeyHelper is headless-only, like the old env token).

=== UNKNOWN 2 — billing / identity ===
  You supplied a key of type: \$PROBE_ANTHROPIC_KEY (sk-ant-api... = an API key).
  A PASS above with an API key confirms the API-KEY (x-api-key, console/pay-per-token)
  path. The OAuth SUBSCRIPTION path stays the separate existing volume login (ADR-0014).

(container "$CTR" is force-removed on exit. Re-run anytime; nothing persists.)
EOF
