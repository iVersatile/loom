#!/bin/sh
# Loom env-wide Claude statusline (base tier). Claude Code pipes session JSON on
# stdin; we print a compact status. Materialized into ~/.claude by `loom build`,
# so it survives a container rebuild (ADR-0001/0006).
dir=${PWD##*/}
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo -)
printf 'loom \xe2\xac\xa1 %s @ %s' "$dir" "$branch"
