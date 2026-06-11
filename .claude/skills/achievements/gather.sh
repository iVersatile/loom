#!/bin/sh
# gather — mechanical inputs for /achievements (T24). READ-ONLY by contract:
# this script never writes the tree; selection, categorization, and narration
# stay with the model (the skill's SKILL.md is the contract).
# usage: gather.sh <since>   — <since> is any `git log --since` string
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-.}"
SINCE="${1:?usage: gather.sh <since>}"
cd "$ROOT"

# git approxidate gotcha: a bare date ("2026-06-11") gets the CURRENT clock
# time, silently emptying the window. Pin bare dates to midnight.
case "$SINCE" in
	[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) SINCE="$SINCE 00:00" ;;
esac

echo "== merged PRs since $SINCE (newest first) =="
git log --merges --since="$SINCE" --pretty='%h %cs %s' main 2>/dev/null \
	|| git log --merges --since="$SINCE" --pretty='%h %cs %s'

echo
echo "== queue rows currently done / in review (docs/PLAN.md fenced section) =="
awk '/BEGIN TACTICAL QUEUE/,/END TACTICAL QUEUE/' docs/PLAN.md \
	| grep -E '\|[[:space:]]*(done|in review)' || true

echo
echo "== OPEN-THREADS headings + status markers =="
grep -E '^## T[0-9]+' docs/OPEN-THREADS.md || true

echo
echo "== lesson headers (docs/LESSONS_LEARNT.md, newest first) =="
grep -E '^## LL-' docs/LESSONS_LEARNT.md || true

echo
echo "== trust flips (.scratch/inbox/flips.log) =="
cat .scratch/inbox/flips.log 2>/dev/null || echo "(none)"

echo
echo "== inbox DONE items (delivery state only; work state is the queue) =="
for f in .scratch/inbox/*.md; do
	[ -r "$f" ] || continue
	echo "-- $f"
	awk '/^--- id:/ { id = $0 } /^status:[[:space:]]*DONE/ { print "  " id }' "$f"
done
