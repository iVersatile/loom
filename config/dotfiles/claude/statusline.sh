#!/bin/sh
# Loom env-wide Claude statusline (base tier). Claude Code pipes session JSON on
# stdin; we print a compact status. Materialized into ~/.claude by `loom build`,
# so it survives a container rebuild (ADR-0001/0006).
#
# Best-effort by design — NO `set -e` (a failing git/jq must degrade the line,
# never blank it). Perf: ONE jq pass (was 10 forks + 10 reparses); git scoped to
# the session cwd in one repo resolution; ONE awk over the role inbox for the
# loom session fields. No agent-written value is ever eval'd or handed to a
# shell (the parked-on injection lesson) — values are read as data only.
input=$(cat)

# --- one jq pass: each field on its OWN LINE; percentages rounded. One field
# per line + successive `read` preserves empty middle fields exactly (a TSV +
# split collapses an empty field and shifts everything after). Values never
# contain newlines (names/numbers/path), so line-splitting is safe. ---
{
  read -r model;      read -r effort;     read -r used
  read -r total_cost; read -r current_dir
  read -r rl5_pct;    read -r rl5_reset;  read -r rl7_pct;   read -r rl7_reset
} <<EOF
$(printf '%s' "$input" | jq -r '
  def pct: if . == null then "" else (. | round | tostring) end;
  .model.display_name // "Unknown Model",
  .effort.level // "",
  (.context_window.used_percentage | pct),
  .cost.total_cost_usd // "",
  .worktree.original_cwd // "",
  (.rate_limits.five_hour.used_percentage | pct),
  .rate_limits.five_hour.resets_at // "",
  (.rate_limits.seven_day.used_percentage | pct),
  .rate_limits.seven_day.resets_at // ""
' 2>/dev/null)
EOF
current_dir="${current_dir:-$PWD}"

GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
GREY='\033[90m'
RESET='\033[0m'

# threshold_color PCT -> emits the ANSI code (literal; the caller's printf %b
# interprets it). Green < 70 <= yellow < 90 <= red.
threshold_color() {
  if   [ "$1" -ge 90 ] 2>/dev/null; then printf '%s' "$RED"
  elif [ "$1" -ge 70 ] 2>/dev/null; then printf '%s' "$YELLOW"
  else printf '%s' "$GREEN"
  fi
}

make_bar() {
  pct="$1"
  width=10
  filled=$(( pct * width / 100 ))
  bar=""
  i=0
  while [ $i -lt $filled ]; do bar="${bar}█"; i=$(( i + 1 )); done
  while [ $i -lt $width ];  do bar="${bar}░"; i=$(( i + 1 )); done
  printf "%s" "$bar"
}

format_rl() {
  pct="$1"
  reset_ts="$2"
  label="$3"
  [ -z "$pct" ] && return
  color=$(threshold_color "$pct")
  reset_time=$(date -r "$reset_ts" "+%-I:%M%p" 2>/dev/null || date -d "@$reset_ts" "+%-I:%M%p" 2>/dev/null)
  bar=$(make_bar "$pct")
  # %b interprets the ANSI escapes; data is the arg (not the format), so a stray
  # % or \ in a value can't be read as a printf directive.
  printf '%b' "${color}${label} ${bar} ${pct}% resets ${reset_time}${RESET}"
}

# Context window as a threshold-colored bar (was a bare percent).
if [ -n "$used" ]; then
  ctx_str=$(printf '%b' "$(threshold_color "$used")$(make_bar "$used") ${used}%${RESET}")
else
  ctx_str="0%"
fi

# Cost.
if [ -n "$total_cost" ]; then
  block_str="\$$(awk "BEGIN { printf \"%.2f\", $total_cost }")"
else
  block_str="\$0.00"
fi

# Rate limit (5h; 7d available but off by default).
rate_limit_str="$(format_rl "$rl5_pct" "$rl5_reset" "5h")"
# rate_limit_str="${rate_limit_str}$(format_rl "$rl7_pct" "$rl7_reset" "7d")"

# --- git: one repo resolution, scoped to the session cwd; +staged ~modified;
# ⚠️ footgun when on main/master with uncommitted changes (branch-guard blocks). ---
git_str="no branch"
dir_display=$(basename "$current_dir")
repo_root=$(git -C "$current_dir" rev-parse --show-toplevel 2>/dev/null)
if [ -n "$repo_root" ]; then
  dir_display=$(basename "$repo_root")
  branch=$(git -C "$current_dir" branch --show-current 2>/dev/null)
  [ -z "$branch" ] && branch=$(git -C "$current_dir" rev-parse --abbrev-ref HEAD 2>/dev/null)
  staged=$(git -C "$current_dir" diff --cached --numstat 2>/dev/null | wc -l | tr -d ' ')
  modified=$(git -C "$current_dir" diff --numstat 2>/dev/null | wc -l | tr -d ' ')
  git_str="$branch"
  [ "${staged:-0}" -gt 0 ]   && git_str="${git_str} $(printf '%b' "${GREEN}+${staged}${RESET}")"
  [ "${modified:-0}" -gt 0 ] && git_str="${git_str} $(printf '%b' "${YELLOW}~${modified}${RESET}")"
  case "$branch" in
    main|master) [ $(( ${staged:-0} + ${modified:-0} )) -gt 0 ] && \
      git_str="${git_str} $(printf '%b' "${RED}⚠️${RESET}")" ;;
  esac
fi

# --- loom session fields: AUTOPILOT, role, queued/parked, HALT. Only on a loom
# repo with the role's inbox. ONE awk pass; values read as DATA (never eval'd). ---
loom_str=""
if [ -n "$repo_root" ]; then
  # Role source = the declarative /var/lib/loom/role marker (loom.yml role:, PR4) —
  # the SAME source the drain-guard reads (adv-066), so the glyph and the drain
  # never disagree and launching a seat needs NO `export` (the marker is per-seat:
  # Writer=loom-author, advisor=loom-advisor). LOOM_SESSION_ROLE stays env-FIRST as
  # the drain-guard's demoted override/test-seam (NOT a launch ritual);
  # LOOM_ROLE_MARKER overrides the marker path for tests only. Read as DATA, never
  # eval'd; the marker is root-owned + charset-validated at write (non-forgeable).
  role="${LOOM_SESSION_ROLE:-$(cat "${LOOM_ROLE_MARKER:-/var/lib/loom/role}" 2>/dev/null || echo loom-author)}"
  inbox="$repo_root/.scratch/inbox/${role}.md"
  if [ -f "$inbox" ]; then
    { read -r ap; read -r queued; read -r parked; } <<EOF2
$(awk 'BEGIN{ap="";q=0;p=0}
  /^AUTOPILOT:/ && ap=="" { ap=$2 }
  /^status: QUEUED/ { q++ }
  /^status: PARKED/ { p++ }
  END { print ap; print q+0; print p+0 }' "$inbox" 2>/dev/null)
EOF2
    if [ "$ap" = "on" ]; then
      ap_str=$(printf '%b' "${GREEN}🟢AUTO${RESET}")
    else
      ap_str=$(printf '%b' "${GREY}⚪AUTO${RESET}")
    fi
    case "$role" in
      loom-author)  rg="✍️" ;;
      loom-advisor) rg="🧭" ;;
      *)            rg="👤" ;;
    esac
    work="$rg"
    [ "${queued:-0}" -gt 0 ] && work="$work 📥$queued"
    [ "${parked:-0}" -gt 0 ] && work="$work ⏸️$parked"
    [ -e "$repo_root/.scratch/inbox/HALT" ] && work="$work $(printf '%b' "${RED}🛑HALT${RESET}")"
    loom_str=" | ${ap_str} | ${work}"
  fi
fi

if [ -n "$effort" ]; then
  printf "🤖 %s | 💪 %s | 🧠 %s | 💰 %s | ⏱️ %s\n📁 %s | 🌿 %s%s" "$model" "$effort" "$ctx_str" "$block_str" "$rate_limit_str" "$dir_display" "$git_str" "$loom_str"
else
  printf "🤖 %s | 🧠 %s | 💰 %s | ⏱️ %s\n📁 %s | 🌿 %s%s" "$model" "$ctx_str" "$block_str" "$rate_limit_str" "$dir_display" "$git_str" "$loom_str"
fi
