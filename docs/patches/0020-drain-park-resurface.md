# Prepared patch — drain-inbox.sh: PARK / re-surface / supersede (ADR-0020 R6)

**Trust path** (`.claude/hooks/**`) → **human-applied**. This is the R6 slice of
ADR-0020's build. The predicate evaluator is lifted verbatim from the proven
prototype `.scratch/proto/resurface-safe.sh` (11/11 + `injection-test.sh`:
no `PWNED`). Integration (re-surface flip, supersede-skip, age-escalate) is
**prepared for review + the author's guard tests** (envelope adv-054) — apply
*after* those tests are green.

## How to apply
```sh
# 1. replace the hook with the content below
#    (review the diff first; it changes the awk picker substantially)
# 2. commit on a branch (trust path needs the audited override):
ALLOW_TRUST_CHANGE=1 git commit -m "feat: drain — PARK/re-surface/supersede (ADR-0020 R6, human-applied)"
# 3. the author's guard suite (internal/guard, adv-054) must pass.
```

## What changes vs the current hook
- **Supersede-skip:** an item with `superseded-by: <id>` is never delivered (the
  staleness gap; specimen adv-049).
- **Re-surface (POLL):** a `status: PARKED` item whose `parked-on:` predicate is
  cleared is treated as deliverable and flipped `PARKED → TAKEN` on pick.
- **Fixed, fail-closed, never-eval'd predicates:** `exists:` / `pr-merged:` /
  `item-status:<id>=<STATUS>`; unknown kind or malformed value = fail-closed
  (stay PARKED). No value ever reaches a shell.
- **Max-park-age → ESCALATE (never drop):** a PARKED item with
  `now - parked-at > MAX_PARK_AGE` and dep uncleared is surfaced in the
  continuation report (and emits one bounded escalation turn if nothing else is
  deliverable). Missing `parked-at` = fail-safe park.
- **R8 cost:** one local merged-PR cache refresh per drain (squash-merge
  subjects `(#NNN)`), not a per-item / network lookup.

## Validation + schema requirement
Smoke-tested in isolation (`.scratch/proto/picker.awk` over a crafted inbox):
re-surface picks the cleared PARKED item (`WASPARKED=1`), an over-age uncleared
park ESCALATEs, a superseded item and a fresh uncleared park are skipped — all
correct. **The test surfaced a hard schema rule** (the parser only reads header
fields before `status:`): **`parked-on` / `parked-at` / `superseded-by` MUST
precede the `status:` line** (same block as `serves`/`kind`). The author's
envelope (adv-054) documents this in the inbox schema and the guard suite asserts
it. Full end-to-end validation (Stop-hook JSON + the real inbox) is the author's
guard tests — apply this patch only once they're green.

## Open policy note (for review)
The escalation here is the *simplest bounded* version: over-age parks are added
to the report note, and if nothing else is deliverable they emit ONE block
(bounded by the 3/drain budget so it cannot spin). A richer tier (TICKET to
standup vs PAGE) can follow once observed.

## Full proposed `.claude/hooks/drain-inbox.sh`
```sh
#!/bin/sh
# drain-inbox — Stop hook (T21 + ADR-0020): when AUTOPILOT is on, don't stop —
# take the next ELIGIBLE item. Eligible = QUEUED, or PARKED whose `parked-on:`
# dependency has cleared (re-surface). Superseded items are skipped; over-age
# parks are escalated, never dropped. The human stops being the message bus AND
# stops being the wake/relay for blocked work (the closed loop, ADR-0020).
#
# Hard guards (docs/TEAM.md "Cross-session transport"):
#   (a) orphan refusal — serves: must match a tactical-queue row, else skipped+flagged;
#   (b) design-envelope legalization — kind: design must carry "thread stub";
#   (c) drain budget — max 3 chained items per drain;
#   (d) never-auto permission floor untouched — this hook only decides stop-vs-continue.
# ADR-0020 additions:
#   (e) PARKED re-surface — parked-on cleared => deliverable (flip PARKED->TAKEN);
#   (f) supersede-skip — superseded-by present => never delivered;
#   (g) predicate vocabulary FIXED + fail-closed + NEVER eval'd (exists:/pr-merged:/
#       item-status:); unknown/malformed => stay PARKED;
#   (h) max-park-age => ESCALATE (surface), never auto-drop.
# Conservative parsing: anything malformed falls through to a normal stop.
#
# Inbox item header fields: id, from, serves, kind, [parked-on], [parked-at],
#   [superseded-by], status: WAITING|QUEUED|PARKED|TAKEN|DONE, <body>.
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-.}"
INBOX="$ROOT/.scratch/inbox/loom-author.md"
QUEUE="$ROOT/docs/PLAN.md"
COUNT_FILE="$ROOT/.scratch/inbox/.drain-count"
MERGED="$ROOT/.scratch/inbox/.merged-prs"
MAX_PARK_AGE=86400  # 24h; over this + dep uncleared => escalate (ADR-0020 R4)

command -v jq >/dev/null 2>&1 || exit 0
[ -r "$INBOX" ] || exit 0
[ -r "$QUEUE" ] || exit 0

# Role guard (LL-011): own loom-author's inbox only.
role="${LOOM_SESSION_ROLE:-}"
if [ -z "$role" ] && [ "$(id -un 2>/dev/null)" = "root" ]; then
	role="loom-author"
fi
[ "$role" = "loom-author" ] || exit 0

# Kill-switch (T23): HALT gates ALL drains, checked before AUTOPILOT.
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
if [ "$count" -ge 3 ]; then
	rm -f "$COUNT_FILE"
	exit 0
fi

# ADR-0020 R8: refresh the local merged-PR cache ONCE per drain (squash-merge
# subjects carry "(#NNN)"). pr-merged: reads this file; never a network call.
git -C "$ROOT" log --format=%s -n 500 2>/dev/null | grep -oE '\(#[0-9]+\)' | tr -dc '0-9\n' > "$MERGED" 2>/dev/null || : > "$MERGED"
NOW=$(date +%s 2>/dev/null || echo 0)

# Pick the first eligible item; collect skipped-orphans and over-age escalations.
pick=$(awk -v queue="$QUEUE" -v merged="$MERGED" -v inbox="$INBOX" -v now="$NOW" -v maxage="$MAX_PARK_AGE" '
	# --- fixed, fail-closed predicate evaluator (from resurface-safe.sh; no shell) ---
	function exists(path,  r){ r=(getline _ < path); close(path); return (r>=0) }
	function pr_merged(n,  line,hit){ if(n !~ /^[0-9]+$/) return 0; hit=0;
		while((getline line < merged)>0) if(line==n) hit=1; close(merged); return hit }
	function item_status(spec,  k,wid,wst,line,cur,curid,a,b){ k=index(spec,"=");
		if(k==0) return 0; wid=substr(spec,1,k-1); wst=substr(spec,k+1);
		if(wid !~ /^[a-z0-9-]+$/ || wst !~ /^[A-Z-]+$/) return 0;
		cur=""; curid="";
		while((getline line < inbox)>0){
			if(line ~ /^--- id:/){ split(line,a," "); curid=a[3]; cur="" }
			else if(line ~ /^status:/){ split(line,b," "); if(curid==wid) cur=b[2] } }
		close(inbox); return (cur==wst) }
	function cleared(pred,  k,kind,val){ if(pred=="") return 1;
		k=index(pred,":"); if(k==0) return 0;
		kind=substr(pred,1,k-1); val=substr(pred,k+1);
		if(kind=="exists")      return exists(val);
		if(kind=="pr-merged")   return pr_merged(val);
		if(kind=="item-status") return item_status(val);
		return 0 }   # unknown kind => fail-closed
	function reset(){ id="";serves="";kind="task";status="";body="";bodyhasstub=0;inbody=0;parkedon="";parkedat="";superseded="" }
	function flush(  ok,found,qline,deliver,resurfaced){
		if(id=="") { reset(); return }
		if(superseded != "") { reset(); return }                 # (f) supersede-skip
		deliver=0; resurfaced=0
		if(status=="QUEUED") deliver=1
		else if(status=="PARKED"){                               # (e) re-surface / (h) age
			if(cleared(parkedon)){ deliver=1; resurfaced=1 }
			else { if(parkedat ~ /^[0-9]+$/ && (now-parkedat)>maxage) escalate=escalate (escalate==""?"":",") id; reset(); return }
		} else { reset(); return }                              # WAITING/TAKEN/DONE
		if(kind=="fyi" || kind=="draft"){ reset(); return }
		ok=1
		if(serves=="") ok=0
		else { found=0; while((getline qline < queue)>0) if(index(qline,serves)>0 && qline ~ /^\|/) found=1; close(queue); if(!found) ok=0 }
		if(ok && kind=="design" && bodyhasstub==0) ok=0
		if(ok && picked==""){ picked=id; pbody=body; pserves=serves; pkind=kind; pwasparked=resurfaced }
		else if(!ok) skipped=skipped (skipped==""?"":",") id
		reset()
	}
	BEGIN { reset(); picked=""; skipped=""; escalate="" }
	/^--- id:/ { flush(); id=$0; sub(/^--- id:[[:space:]]*/,"",id); next }
	id!="" && !inbody && /^from:/          { next }
	id!="" && !inbody && /^serves:/        { serves=$0; sub(/^serves:[[:space:]]*/,"",serves); next }
	id!="" && !inbody && /^kind:/          { kind=$0; sub(/^kind:[[:space:]]*/,"",kind); next }
	id!="" && !inbody && /^parked-on:/     { parkedon=$2; next }
	id!="" && !inbody && /^parked-at:/     { parkedat=$2; next }
	id!="" && !inbody && /^superseded-by:/ { superseded=$2; next }
	id!="" && !inbody && /^status:/        { status=$0; sub(/^status:[[:space:]]*/,"",status); inbody=1; next }
	id!="" && inbody { body=body $0 "\n"; if(tolower($0) ~ /thread stub/) bodyhasstub=1; next }
	END {
		flush()
		if(picked!=""){ print "ID\t" picked; print "SERVES\t" pserves; print "WASPARKED\t" pwasparked; printf "%s","BODY\t"; print pbody }
		if(skipped!="") print "SKIPPED\t" skipped
		if(escalate!="") print "ESCALATE\t" escalate
	}
' "$INBOX" 2>/dev/null) || exit 0

pid=$(printf '%s\n' "$pick" | sed -n 's/^ID\t//p' | head -1)
pesc=$(printf '%s\n' "$pick" | sed -n 's/^ESCALATE\t//p' | head -1)

if [ -z "$pid" ]; then
	# Nothing deliverable. (h) If over-age parks exist, emit ONE bounded
	# escalation (counts against the 3/drain budget so it cannot spin).
	[ -z "$pesc" ] && exit 0
	echo $((count + 1)) > "$COUNT_FILE"
	reason="AUTOPILOT drain — NO deliverable item, but PARKED items are over max age ($pesc). Their parked-on dependency has not cleared in >24h. INVESTIGATE: resolve, re-route, or drop (never silently). The never-auto floor still applies."
	jq -n --arg r "$reason" '{decision: "block", reason: $r}'
	exit 0
fi

pserves=$(printf '%s\n' "$pick" | sed -n 's/^SERVES\t//p' | head -1)
pskipped=$(printf '%s\n' "$pick" | sed -n 's/^SKIPPED\t//p' | head -1)
pbody=$(printf '%s\n' "$pick" | sed -n '/^BODY\t/,$p' | sed '1s/^BODY\t//')

# Mark the item TAKEN — matches QUEUED OR PARKED (a re-surfaced pick).
tmp="$INBOX.tmp.$$"
awk -v target="$pid" '
	/^--- id:/ { cur = $0; sub(/^--- id:[[:space:]]*/, "", cur) }
	cur == target && /^status:[[:space:]]*(QUEUED|PARKED)/ { print "status: TAKEN"; next }
	{ print }
' "$INBOX" > "$tmp" && mv "$tmp" "$INBOX"

echo $((count + 1)) > "$COUNT_FILE"

note=""
[ -n "$pskipped" ] && note="$note Orphan/illegal items skipped (flag these in your report): $pskipped."
[ -n "$pesc" ] && note="$note Over-age PARKED items needing attention: $pesc."
reason="AUTOPILOT drain — inbox item $pid ($((count + 1))/3 this drain), serves queue row: $pserves.$note
Do the work it describes; the never-auto permission floor still applies.
When shipped: flip the queue row in that PR, then set item $pid to DONE in .scratch/inbox/loom-author.md.
--- item body ---
$pbody"

jq -n --arg r "$reason" '{decision: "block", reason: $r}'
```
