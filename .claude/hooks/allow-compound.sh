#!/bin/sh
# allow-compound — PreToolUse(Bash) hook: auto-approve compound commands iff
# EVERY segment matches the repo allow set (.claude/settings.json), killing
# the "allowed && allowed → prompt" class without enumerating combinations.
#
# Conservative by construction: any construct this script can't confidently
# split or match falls through to the normal permission flow (exit 0 with no
# output = no opinion). It can therefore only ever APPROVE a chain whose every
# piece was already individually approved policy; it never denies, and the
# deny floor is consulted first — a segment matching a deny prefix falls
# through so the harness's deny rule (which binds in all modes) fires.
set -eu

command -v jq >/dev/null 2>&1 || exit 0
settings="${CLAUDE_PROJECT_DIR:-.}/.claude/settings.json"
[ -r "$settings" ] || exit 0

cmd=$(jq -r '.tool_input.command // empty' 2>/dev/null) || exit 0
[ -n "$cmd" ] || exit 0

# Splitting + bail-outs live in THE shared splitter (T29 amendment 5: one
# splitter; two splitters drift). Uncertainty (quotes, expansion, redirection,
# comments, newlines, grouping) -> split_segments returns 1 -> no opinion.
. "${CLAUDE_PROJECT_DIR:-.}/config/hooks/segment-split"
segs=$(split_segments "$cmd") || exit 0
[ -n "$segs" ] || exit 0

# Allow/deny prefixes from the same settings file the harness reads — one
# source of truth. "Bash(x:*)" → prefix x; "Bash(x)" → exact x.
allows=$(jq -r '.permissions.allow[]? | select(startswith("Bash("))' "$settings")
denies=$(jq -r '.permissions.deny[]?  | select(startswith("Bash("))' "$settings")

# matches LIST SEGMENT → 0 if segment matches any rule in LIST.
# NOTE: no bare `[ … ] && …` lists in here — under set -e a false test in an
# AND-list kills the subshell (the LL-006/010 family of silent test bugs).
matches() {
  _rules=$1 _seg=$2
  printf '%s\n' "$_rules" | while IFS= read -r _r; do
    if [ -z "$_r" ]; then continue; fi
    _body=${_r#Bash(}; _body=${_body%)}
    case "$_body" in
      *:\*)
        _p=${_body%:\*}
        if [ "$_seg" = "$_p" ]; then echo hit; break; fi
        case "$_seg" in "$_p "*) echo hit; break ;; esac
        ;;
      *)
        if [ "$_seg" = "$_body" ]; then echo hit; break; fi
        ;;
    esac
  done | grep -q hit
}

# Every shared-splitter segment must clear deny then match allow.
printf '%s\n' "$segs" \
  | {
    any=0
    while IFS= read -r seg; do
      [ -n "$seg" ] || continue
      any=1
      if matches "$denies" "$seg"; then exit 1; fi # deny floor: fall through
      if ! matches "$allows" "$seg"; then exit 1; fi
    done
    [ "$any" = 1 ] || exit 1
    exit 0
  } || exit 0

printf '%s' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"every segment matches the repo allow set (.claude/settings.json)"}}'
