# ADR-0002 — Thin playbook + resolver + lockfile
**Date:** 2026-06-05   **Status:** Accepted

## Context
Config must be managed "like a dependency," editable by humans and AI, and able
to evolve as tools/stacks/workflows change. Mature tools split into two camps:
thin (Devbox `devbox.json`, Dev Containers) which dominate adoption, and thick
(Nix derivations) which are powerful but gated by a learning curve.

## Decision
The playbook is **thin** (declares intent: `python@3.11`, tool names, overlay,
rules-by-reference), interpreted by a **resolver** in the build engine that maps
intent to concrete installs, and pinned by a generated **lockfile** for exact
reproducibility. Desired-state/declarative — the engine diffs and converges.

## Alternatives considered
- **Thick playbook** (exact packages, hook bodies, file contents) — maximally
  flexible but large, hard for novices and AI to author. Rejected as default.
- **Imperative step list** — can't express "this tool should now be absent";
  re-runs double-apply; `update` becomes teardown+rebuild. Rejected.

## Consequences
- Positive: gentle for scenario-1 novice and AI authors; idempotent reconcile;
  removal/drift-correction trivial; reproducible via lock; matches market winners.
- Trade-offs: requires a resolver layer (intent → concrete) the engine must own.
- Revisit if: a use case needs precision the thin schema can't express → use the
  per-entry pin/escape-hatch before thickening the whole schema.
