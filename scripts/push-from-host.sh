#!/bin/sh
# push-from-host.sh — push loom-author's local branches and hint the PRs
# Purpose: the outward-ops half of the T18 ritual. The agent commits
#          in-container but has no VCS credentials (by design until the
#          T15-successor credential ADR); this script is the Mac-side step:
#          list every unpushed branch, push on confirmation, print PR hints.
# Origin : T18 (multi-agent dogfood) — "agent can commit but not ship"; the
#          ritual was prose in the handoff until it recurred enough to be a
#          script (T17 lifecycle: recurring).
# Reuse  : recurring — after every in-container working session that leaves
#          branches in the handoff addendum.
# Runs-on: the Mac host (needs git push credentials + gh). Refuses containers.
# Lifecycle: recurring → verb-candidate (an outward-ops verb is plausible once
#          T15 gives the agent a leak-free credential path; this script then
#          dies with a pointer, per scripts/README.md).
set -eu

if [ -f /.dockerenv ]; then
  echo "ERROR: this looks like a container ($(whoami)@$(hostname)) — run on the Mac host" >&2
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

unpushed=$(git log --branches --not --remotes --format='%D' \
  | tr ',' '\n' | sed 's/^ *//' | grep -vE '^(HEAD|tag:|$)' | sort -u)

if [ -z "$unpushed" ]; then
  echo "nothing to push: every local branch is on the remote."
  exit 0
fi

echo "unpushed branches:"
echo "$unpushed" | sed 's/^/  /'
echo

for b in $unpushed; do
  printf 'push %s? [y/N] ' "$b"
  read -r ans
  case "$ans" in
    y|Y)
      git push -u origin "$b"
      if command -v gh >/dev/null 2>&1; then
        echo "  PR: gh pr create --base main --head $b --fill"
      else
        echo "  PR: open a pull request for $b (gh not found)"
      fi
      ;;
    *) echo "  skipped $b" ;;
  esac
done

echo
echo "done. merged-branch cleanup hint:"
echo "  git fetch --prune && git branch --merged main | grep -v main"
