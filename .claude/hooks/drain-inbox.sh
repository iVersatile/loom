#!/bin/sh
# drain-inbox — Stop hook (T21): when AUTOPILOT is on and the role's inbox has
# a QUEUED item, don't stop — take the next item. The human stops being the
# message bus; the tree carries the mail.
#
# Hard guards (all four, docs/TEAM.md "Cross-session transport"):
#   (a) orphan refusal — an item whose `serves:` matches no tactical-queue row
#       is skipped and flagged in the continuation report, never worked;
#   (b) design-envelope legalization — `kind: design` must instruct logging a
#       thread stub before work proceeds ("thread stub" in the body), else it
#       is treated as an orphan: ephemeral must carry durable's birth
#       certificate, or it doesn't ride;
#   (c) drain budget — max 3 chained items per drain, then stop regardless;
#   (d) the never-auto permission floor is untouched — this hook only decides
#       stop-vs-continue; protected operations still prompt mid-drain.
# Conservative parsing: anything malformed falls through to a normal stop.
#
# Inbox format (.scratch/inbox/<role>.md — untracked; mail, not memory):
#   AUTOPILOT: off|on          (header, default off; the human flips it)
#   --- id: NNN
#   from: <role>
#   serves: <queue-row task text fragment>
#   kind: task|design          (optional; default task)
#   status: WAITING|QUEUED|TAKEN|DONE
#   <body…>
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-.}"
INBOX="$ROOT/.scratch/inbox/loom-author.md"
QUEUE="$ROOT/docs/PLAN.md"
COUNT_FILE="$ROOT/.scratch/inbox/.drain-count"

command -v jq >/dev/null 2>&1 || exit 0
[ -r "$INBOX" ] || exit 0
[ -r "$QUEUE" ] || exit 0

# Role guard (LL-011): this hook registers REPO-LEVEL, so it fires in every
# session on the shared tree — advisor sessions included — but it owns
# loom-author's inbox ONLY. Resolve the session's role and no-op for anyone
# else, or the wrong session drains the Writer's cargo (the 2026-06-11
# incident: an advisor stop flipped item 001 to TAKEN).
# Resolution: LOOM_SESSION_ROLE env wins (explicit marker + test seam);
# otherwise whoami is ground truth — root IS loom-author in loom-dev today.
# Revisit at T10 (non-root container): the fallback must become a materialized
# role marker, not a uid check.
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ "$(id -un 2>/dev/null)" = "root" ]; then
	role="loom-author"
fi
[ "$role" = "loom-author" ] || exit 0

# AUTOPILOT gate: default off; anything but exactly "on" allows a normal stop.
ap=$(sed -n 's/^AUTOPILOT:[[:space:]]*//p' "$INBOX" | head -1)
[ "$ap" = "on" ] || exit 0

# Drain budget (guard c): the counter persists across chained stops; a stop we
# ALLOW resets it, so the next drain starts fresh.
input=$(cat)
active=$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)
count=0
if [ "$active" = "true" ] && [ -r "$COUNT_FILE" ]; then
	count=$(cat "$COUNT_FILE" 2>/dev/null || echo 0)
	case "$count" in *[!0-9]*) count=0 ;; esac
fi
if [ "$count" -ge 3 ]; then
	rm -f "$COUNT_FILE"
	exit 0 # budget exhausted: stop regardless (guard c)
fi

# Find the first QUEUED item that clears guards (a) and (b); collect skipped
# orphan ids so the continuation report can flag them (guard a, "flag").
pick=$(awk -v queue="$QUEUE" '
	function flush() {
		if (id == "" || status != "QUEUED") { reset(); return }
		ok = 1
		# guard (a): serves must be non-empty and match a queue row.
		if (serves == "") ok = 0
		else {
			found = 0
			while ((getline qline < queue) > 0)
				if (index(qline, serves) > 0 && qline ~ /^\|/) found = 1
			close(queue)
			if (!found) ok = 0
		}
		# guard (b): design envelopes must carry the thread-stub instruction.
		if (ok && kind == "design" && bodyhasstub == 0) ok = 0
		if (ok && picked == "") { picked = id; pbody = body; pserves = serves; pkind = kind }
		else if (!ok) skipped = skipped (skipped == "" ? "" : ",") id
		reset()
	}
	function reset() { id=""; serves=""; kind="task"; status=""; body=""; bodyhasstub=0; inbody=0 }
	BEGIN { reset(); picked = ""; skipped = "" }
	/^--- id:/ { flush(); id = $0; sub(/^--- id:[[:space:]]*/, "", id); next }
	id != "" && !inbody && /^from:/   { next }
	id != "" && !inbody && /^serves:/ { serves = $0; sub(/^serves:[[:space:]]*/, "", serves); next }
	id != "" && !inbody && /^kind:/   { kind = $0; sub(/^kind:[[:space:]]*/, "", kind); next }
	id != "" && !inbody && /^status:/ { status = $0; sub(/^status:[[:space:]]*/, "", status); inbody = 1; next }
	id != "" && inbody {
		body = body $0 "\n"
		if (tolower($0) ~ /thread stub/) bodyhasstub = 1
		next
	}
	END {
		flush()
		if (picked != "") {
			print "ID\t" picked
			print "SERVES\t" pserves
			printf "%s", "BODY\t"; print pbody
		}
		if (skipped != "") print "SKIPPED\t" skipped
	}
' "$INBOX" 2>/dev/null) || exit 0

pid=$(printf '%s\n' "$pick" | sed -n 's/^ID\t//p' | head -1)
[ -n "$pid" ] || exit 0 # nothing eligible: normal stop
pserves=$(printf '%s\n' "$pick" | sed -n 's/^SERVES\t//p' | head -1)
pskipped=$(printf '%s\n' "$pick" | sed -n 's/^SKIPPED\t//p' | head -1)
pbody=$(printf '%s\n' "$pick" | sed -n '/^BODY\t/,$p' | sed '1s/^BODY\t//')

# Mark the item TAKEN in our own inbox (the role updates only its own inbox's
# statuses — TEAM.md cross-agent write rule). Targeted flip, conservative.
tmp="$INBOX.tmp.$$"
awk -v target="$pid" '
	/^--- id:/ { cur = $0; sub(/^--- id:[[:space:]]*/, "", cur) }
	cur == target && /^status:[[:space:]]*QUEUED/ { print "status: TAKEN"; next }
	{ print }
' "$INBOX" > "$tmp" && mv "$tmp" "$INBOX"

echo $((count + 1)) > "$COUNT_FILE"

note=""
[ -n "$pskipped" ] && note=" Orphan/illegal items skipped (flag these in your report): $pskipped."
reason="AUTOPILOT drain — inbox item $pid ($((count + 1))/3 this drain), serves queue row: $pserves.$note
Do the work it describes; the never-auto permission floor still applies.
When shipped: flip the queue row in that PR (work state lives in the queue,
not here), then set item $pid to DONE in .scratch/inbox/loom-author.md.
--- item body ---
$pbody"

jq -n --arg r "$reason" '{decision: "block", reason: $r}'
