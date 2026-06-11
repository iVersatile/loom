#!/bin/sh
# dispatch-inbox — promote pre-authored inbox items when their condition holds
# Purpose: the conditional half of the T21 transport. Items authored ahead of
#          time carry `status: WAITING` + an `after: <condition>` line; this
#          script checks each condition and flips WAITING -> QUEUED so the
#          Stop-hook drain (or a human glance) picks them up. Conditions:
#            after: pr-merged #NN     — GitHub PR NN is merged (gh)
#            after: row-done <text>   — the tactical-queue row containing
#                                       <text> has status done
# Origin : T21 (cross-session task transport) — the human was the message bus;
#          "when X merges, tell Writer Y" is a condition check, not judgment.
# Reuse  : recurring — run alongside push-from-host.sh on the human's word.
# Runs-on: the Mac host (needs gh for pr-merged checks; row-done works
#          anywhere). Refuses nothing in-container except gh-dependent checks.
# Lifecycle: recurring -> drain-integrated or loom-native dispatch later (T21
#          promote-to); dies with a pointer when that lands.
set -eu

ROOT="$(git rev-parse --show-toplevel)"
QUEUE="$ROOT/docs/PLAN.md"
promoted=0

# check_after CONDITION -> 0 if it holds. Unknown condition = does NOT hold
# (conservative: a typo never promotes).
check_after() {
	case "$1" in
	"pr-merged #"*)
		command -v gh >/dev/null 2>&1 || { echo "skip (no gh): $1" >&2; return 1; }
		n=${1#pr-merged \#}
		[ "$(gh pr view "$n" --json state -q .state 2>/dev/null)" = "MERGED" ]
		;;
	"row-done "*)
		frag=${1#row-done }
		grep -F "$frag" "$QUEUE" | grep -q "| done"
		;;
	*) return 1 ;;
	esac
}

for inbox in "$ROOT"/.scratch/inbox/*.md; do
	[ -r "$inbox" ] || continue
	# Collect ids whose condition holds.
	ids=""
	cur=""; after=""; status=""
	while IFS= read -r line || [ -n "$line" ]; do
		case "$line" in
		"--- id:"*)
			if [ -n "$cur" ] && [ "$status" = "WAITING" ] && [ -n "$after" ]; then
				if check_after "$after"; then ids="$ids $cur"; fi
			fi
			# Conservative parsing: headers are "key: value", one space.
			cur=${line#"--- id: "}; after=""; status=""
			;;
		"after: "*) after=${line#"after: "} ;;
		"status: "*) status=${line#"status: "} ;;
		esac
	done < "$inbox"
	if [ -n "$cur" ] && [ "$status" = "WAITING" ] && [ -n "$after" ]; then
		if check_after "$after"; then ids="$ids $cur"; fi
	fi

	for id in $ids; do
		tmp="$inbox.tmp.$$"
		awk -v target="$id" '
			/^--- id:/ { c = $0; sub(/^--- id:[[:space:]]*/, "", c) }
			c == target && /^status:[[:space:]]*WAITING/ { print "status: QUEUED"; next }
			{ print }
		' "$inbox" > "$tmp" && mv "$tmp" "$inbox"
		echo "promoted: $(basename "$inbox") item $id"
		promoted=$((promoted + 1))
	done
done

[ "$promoted" -gt 0 ] || echo "nothing to promote."
exit 0
