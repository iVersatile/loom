#!/bin/sh
# Loom env-wide Claude statusline (base tier). Claude Code pipes session JSON on
# stdin; we print a compact status. Materialized into ~/.claude by `loom build`,
# so it survives a container rebuild (ADR-0001/0006).
#
# Best-effort by design — NO `set -e` (a failing git/jq must degrade the line,
# never blank it). Perf: ONE jq pass (was 10 forks + 10 reparses) and git
# scoped to the session cwd in one repo resolution (was a mix of process-cwd and
# original_cwd — could show the wrong repo).
input=$(cat)

# --- one jq pass: each field on its OWN LINE; percentages rounded. One field
# per line + successive `read` preserves empty middle fields exactly (a TSV +
# split collapses an empty field and shifts everything after it). Field values
# never contain newlines (names/numbers/path), so line-splitting is safe. ---
{
  read -r model;      read -r effort;     read -r used;      read -r worktree
  read -r total_cost; read -r current_dir
  read -r rl5_pct;    read -r rl5_reset;  read -r rl7_pct;   read -r rl7_reset
} <<EOF
$(printf '%s' "$input" | jq -r '
  def pct: if . == null then "" else (. | round | tostring) end;
  .model.display_name // "Unknown Model",
  .effort.level // "",
  (.context_window.used_percentage | pct),
  .worktree.name // "",
  .cost.total_cost_usd // "",
  .worktree.original_cwd // "",
  (.rate_limits.five_hour.used_percentage | pct),
  .rate_limits.five_hour.resets_at // "",
  (.rate_limits.seven_day.used_percentage | pct),
  .rate_limits.seven_day.resets_at // ""
' 2>/dev/null)
EOF
current_dir="${current_dir:-$PWD}"

if [ -n "$used" ]; then usage_str="${used}%"; else usage_str="0%"; fi
if [ -n "$worktree" ]; then worktree_str="$worktree"; else worktree_str="no worktree"; fi

GREEN='\033[32m'
YELLOW='\033[33m'
RED='\033[31m'
RESET='\033[0m'

# --- git: one repo resolution, scoped to the session cwd (consistent) ---
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
fi

if [ -n "$total_cost" ]; then
  cost_display=$(awk "BEGIN { printf \"%.2f\", $total_cost }")
  block_str="\$${cost_display}"
else
  block_str="\$0.00"
fi

make_bar() {
  pct="$1"
  width=10
  filled=$(( pct * width / 100 ))
  empty=$(( width - filled ))
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
  if [ "$pct" -ge 90 ]; then color="$RED"
  elif [ "$pct" -ge 70 ]; then color="$YELLOW"
  else color="$GREEN"
  fi
  reset_time=$(date -r "$reset_ts" "+%-I:%M%p" 2>/dev/null || date -d "@$reset_ts" "+%-I:%M%p" 2>/dev/null)
  bar=$(make_bar "$pct")
  # %b interprets the ANSI escapes; data is passed as the arg (not the format),
  # so a stray % or \ in a value can't be read as a printf directive.
  printf '%b' "${color}${label} ${bar} ${pct}% resets ${reset_time}${RESET}"
}

rate_limit_str=""
rate_limit_str="${rate_limit_str}$(format_rl "$rl5_pct" "$rl5_reset" "5h")"
# rate_limit_str="${rate_limit_str}$(format_rl "$rl7_pct" "$rl7_reset" "7d")"

if [ -n "$effort" ]; then
  printf "🤖 %s | 💪 %s | 🧠 %s | 💰 %s | ⏱️ %s\n📁 %s | 🌳 %s | 🌿 %s" "$model" "$effort" "$usage_str" "$block_str" "$rate_limit_str" "$dir_display" "$worktree_str" "$git_str"
else
  printf "🤖 %s | 🧠 %s | 💰 %s | ⏱️ %s\n📁 %s | 🌳 %s | 🌿 %s" "$model" "$usage_str" "$block_str" "$rate_limit_str" "$dir_display" "$worktree_str" "$git_str"
fi
