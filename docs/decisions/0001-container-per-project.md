# ADR-0001 — Container-per-project isolation
**Date:** 2026-06-05   **Status:** Accepted

## Context
Loom must run multiple projects with genuinely different stacks side-by-side
(e.g. a Python 3.11 compiler project beside a Go service and a TS app). Sharing
one container forces one toolchain and risks port collisions and version
conflicts that leak across projects.

## Decision
Each project gets its own container, built from a shared base image. The
per-project overlay (stack tools, rules, CI) is declared in that project's
playbook and baked into its container definition — not applied ad-hoc at runtime.

## Alternatives considered
- **One container, projects as folders, overlay by param** — simplest, but
  different stacks collide (tool versions, ports). Rejected for the
  different-stacks requirement.
- **One container, overlay selected by env var** — invisible state; same command
  behaves differently based on an unseen variable. Rejected as most-confusing.

## Consequences
- Positive: true isolation; no cross-talk; independent teardown; matches
  "different stacks side-by-side"; overlay stops being a tracked ad-hoc action.
- Trade-offs: more containers to manage; shared base image needed to avoid bloat.
- Revisit if: projects converge to one stack family (then one-container is simpler).

## Addendum (2026-06-10) — naming, managed-marker, project mount (T11/T13)
- **Naming:** the per-project container is **`<project>-dev`** (`loom-dev`,
  `prompiler-dev`). The original `loom-<project>-dev` template doubled the name
  for the loom project itself (`loom-loom-dev`) and conflated identity with
  display. The "loom-managed" marker is now **docker labels** —
  `loom.managed=true`, `loom.project=<name>` — discoverable via
  `docker ps --filter label=loom.managed=true`.
- **Project mount:** the project root (the directory holding `loom.yml`) is
  bind-mounted **RW** at **`/workspace/<project>`** at create (the devcontainer
  model, ADR-0003) — container-per-project includes the project being *in* the
  container. Create-time-only (docker cannot add `-v` live); changing it
  requires `--force`.
