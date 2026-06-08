#!/bin/sh
# env-wide Claude statusline (base tier) — reads context JSON on stdin.
printf 'loom:%s' "${PWD##*/}"
