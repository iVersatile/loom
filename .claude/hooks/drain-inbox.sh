#!/bin/sh
# drain-inbox — Stop hook (T21 + ADR-0020). THIN orchestrator: the per-item
# PARK / re-surface / supersede / escalate DECISION lives in
# config/hooks/resurface-decide (protect-paths-guarded after patch 0021). This
# hook adds the safety the decision must NOT own — the orphan-guard (serves must
# match a queue row), the design-stub guard, the drain budget, the TAKEN flip,
# the block emit. Conservative: anything malformed falls through to a normal stop.
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-.}"
INBOX="$ROOT/.scratch/inbox/loom-author.md"
QUEUE="$ROOT/docs/PLAN.md"
COUNT_FILE="$ROOT/.scratch/inbox/.drain-count"
MERGED="$ROOT/.scratch/inbox/.merged-prs"
DECIDE="$ROOT/config/hooks/resurface-decide"

command -v jq >/dev/null 2>&1 || exit 0
[ -r "$INBOX" ] || exit 0
[ -r "$QUEUE" ] || exit 0
[ -r "$DECIDE" ] || exit 0

# Role guard (LL-011): own loom-author's inbox only.
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ -r /var/lib/loom/role ]; then role=$(cat /var/lib/loom/role 2>/dev/null); fi
[ "$role" = "loom-author" ] || exit 0

# Kill-switch (T23): HALT gates ALL drains, before AUTOPILOT.
[ -e "$ROOT/.scratch/inbox/HALT" ] && exit 0

# AUTOPILOT gate.
ap=$(sed -n 's/^AUTOPILOT:[[:space:]]*//p' "$INBOX" | head -1)
[ "$ap" = "on" ] || exit 0

# Drain budget (guard c).
input=$(cat)
active=$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)
count=0
if [ "$active" = "true" ] && [ -r "$COUNT_FILE" ]; then
	count=$(cat "$COUNT_FILE" 2>/dev/null || echo 0)
	case "$count" in *[!0-9]*) count=0 ;; esac
fi
if [ "$count" -ge 3 ]; then rm -f "$COUNT_FILE"; exit 0; fi

# R8: refresh the local merged-PR cache once per drain (squash subjects "(#NNN)").
git -C "$ROOT" log --format=%s -n 500 2>/dev/null | grep -oE '\(#[0-9]+\)' | tr -dc '0-9\n' > "$MERGED" 2>/dev/null || : > "$MERGED"

# Pass 1: the GUARDED decision. Deliverable = DELIVER (QUEUED) or RESURFACE
# (parked dep cleared); escalate = over-age parks.
decisions=$(sh "$DECIDE" "$INBOX" "$MERGED" 2>/dev/null) || exit 0
deliverable=$(printf '%s\n' "$decisions" | sed -n 's/^\([^:]*\): \(DELIVER\|RESURFACE\).*$/\1/p' | paste -sd, - 2>/dev/null)
escalate=$(printf '%s\n' "$decisions" | sed -n 's/^\([^:]*\): ESCALATE.*$/\1/p' | paste -sd, - 2>/dev/null)

# Pass 2: pick the first deliverable item that ALSO clears the drain's safety —
# orphan-guard (serves matches a queue row) + design-stub + non-fyi/draft.
pick=$(awk -v queue="$QUEUE" -v deliverable=",${deliverable}," '
	function flush(  ok,found,qline) {
		if (id == "") { reset(); return }
		if (index(deliverable, "," id ",") == 0) { reset(); return }
		if (kind == "fyi" || kind == "draft") { reset(); return }
		ok = 1
		if (serves == "") ok = 0
		else { found = 0; while ((getline qline < queue) > 0) if (index(qline, serves) > 0 && qline ~ /^\|/) found = 1; close(queue); if (!found) ok = 0 }
		if (ok && kind == "design" && bodyhasstub == 0) ok = 0
		if (ok && picked == "") { picked = id; pbody = body; pserves = serves }
		else if (!ok) skipped = skipped (skipped == "" ? "" : ",") id
		reset()
	}
	function reset() { id=""; serves=""; kind="task"; status=""; body=""; bodyhasstub=0; inbody=0 }
	BEGIN { reset(); picked=""; skipped="" }
	/^--- id:/ { flush(); id=$0; sub(/^--- id:[[:space:]]*/,"",id); next }
	id!="" && !inbody && /^from:/   { next }
	id!="" && !inbody && /^serves:/ { serves=$0; sub(/^serves:[[:space:]]*/,"",serves); next }
	id!="" && !inbody && /^kind:/   { kind=$0; sub(/^kind:[[:space:]]*/,"",kind); next }
	id!="" && !inbody && /^status:/ { status=$0; sub(/^status:[[:space:]]*/,"",status); inbody=1; next }
	id!="" && !inbody { next }
	id!="" && inbody { body=body $0 "\n"; if (tolower($0) ~ /thread stub/) bodyhasstub=1; next }
	END {
		flush()
		if (picked != "") { print "ID\t" picked; print "SERVES\t" pserves; printf "%s","BODY\t"; print pbody }
		if (skipped != "") print "SKIPPED\t" skipped
	}
' "$INBOX" 2>/dev/null) || exit 0

pid=$(printf '%s\n' "$pick" | sed -n 's/^ID\t//p' | head -1)

if [ -z "$pid" ]; then
	[ -z "$escalate" ] && exit 0
	echo $((count + 1)) > "$COUNT_FILE"
	reason="AUTOPILOT drain — no deliverable item, but PARKED items are over max age ($escalate); their parked-on dependency has not cleared. INVESTIGATE (resolve / re-route / drop — never silently). The never-auto floor still applies."
	jq -n --arg r "$reason" '{decision:"block", reason:$r}'
	exit 0
fi

pserves=$(printf '%s\n' "$pick" | sed -n 's/^SERVES\t//p' | head -1)
pskipped=$(printf '%s\n' "$pick" | sed -n 's/^SKIPPED\t//p' | head -1)
pbody=$(printf '%s\n' "$pick" | sed -n '/^BODY\t/,$p' | sed '1s/^BODY\t//')

# Flip TAKEN — QUEUED or PARKED (a re-surfaced pick).
tmp="$INBOX.tmp.$$"
awk -v target="$pid" '
	/^--- id:/ { cur=$0; sub(/^--- id:[[:space:]]*/,"",cur) }
	cur==target && /^status:[[:space:]]*(QUEUED|PARKED)/ { print "status: TAKEN"; next }
	{ print }
' "$INBOX" > "$tmp" && mv "$tmp" "$INBOX"

echo $((count + 1)) > "$COUNT_FILE"

note=""
[ -n "$pskipped" ] && note="$note Orphan/illegal items skipped (flag in your report): $pskipped."
[ -n "$escalate" ] && note="$note Over-age PARKED items needing attention: $escalate."
reason="AUTOPILOT drain — inbox item $pid ($((count + 1))/3 this drain), serves queue row: $pserves.$note
Do the work it describes; the never-auto permission floor still applies.
When shipped: flip the queue row in that PR, then set item $pid to DONE in .scratch/inbox/loom-author.md.
--- item body ---
$pbody"

jq -n --arg r "$reason" '{decision:"block", reason:$r}'
