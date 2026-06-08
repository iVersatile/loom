# ADR-0004 — Two-tier config (env-wide base + per-project overlay)
**Date:** 2026-06-05   **Status:** Accepted

## Context
Some configuration is personal/machine-wide and shared across every project
(agent CLIs, global rules, hooks, session memory); some is project-specific
(stack, overlay, ports, project rules). Mixing them pollutes unrelated projects.

## Decision
Two tiers, independently versioned:
- **Base (env-wide)** — `~/.claude/`-style: shared rules, agents, hooks, skills.
- **Overlay (per-project)** — lives in the project repo, versioned with its code.
The engine merges base + stack + overlay at build time (later layer wins for whole
files; lists concatenate).

## Alternatives considered
- **Single flat config** — project specifics leak into every project. Rejected.

## Consequences
- Positive: "my machine setup" and "this project's needs" version independently;
  project config travels with its repo; clean layering for container-per-project.
- Trade-offs: a small, well-defined merge rule must exist and stay simple.
- Revisit if: layering needs richer merge semantics than whole-file + list-concat.
  Known case, deferred: `~/.claude/settings.json` is a single JSON file, so whole-file
  later-wins clobbers rather than key-merges. Phase 1 keeps it base-only (no override);
  key-level JSON merge is a deliberate future addition (SPEC-playbook open question),
  not to be added silently.
