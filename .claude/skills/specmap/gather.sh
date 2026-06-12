#!/bin/sh
# gather — mechanical inputs for /specmap. READ-ONLY: emits the three layers
# (FR registry, threads, spec headings); joins/classification stay with the
# model (SKILL.md is the contract).
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-.}"
cd "$ROOT"

echo "== FR registry (id | sources | tests-present | status) =="
awk '
	/^[[:space:]]*- id:/      { if (id != "") emit(); id = $NF }
	/^[[:space:]]*source:/    { src = $0; sub(/^[[:space:]]*source:[[:space:]]*/, "", src) }
	/^[[:space:]]*tests:/     { tp = ($0 ~ /\[[[:space:]]*\]/) ? "NO-TESTS" : "tests-present" }
	/^[[:space:]]*status:/    { st = $NF }
	function emit() { printf "%s | %s | %s | %s\n", id, src, tp, st; src=""; tp="?"; st="?" }
	END { if (id != "") emit() }
' docs/FR-registry.yml

echo
echo "== threads (heading | status marker) + Pointers lines =="
grep -E '^## T[0-9]+|^Pointers:|^  *Pointers:' docs/OPEN-THREADS.md \
	|| grep -E '^## T[0-9]+' docs/OPEN-THREADS.md

echo
echo "== SPEC-verbs section headings =="
grep -E '^#{2,3} ' docs/SPEC-verbs.md

echo
echo "== SPEC-playbook section headings =="
grep -E '^#{2,3} ' docs/SPEC-playbook.md

echo
echo "== current map header (Generated line + legend, for format reference) =="
head -15 docs/spec-map.md 2>/dev/null || echo "(no existing map)"
