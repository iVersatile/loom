# Architecture Decision Records

One file per decision. Numbered, dated, append-only (supersede rather than edit).
These capture *why*, so future-you and AI contributors don't re-litigate settled
choices.

## Index

- ADR-0001 — Container-per-project isolation
- ADR-0002 — Thin playbook + resolver + lockfile (declarative desired-state)
- ADR-0003 — Devcontainer as input (import-and-enrich), not output
- ADR-0004 — Two-tier config (env-wide base + per-project overlay)
- ADR-0005 — AI agent as first-class user (machine surface + enforced guardrails)
- ADR-0006 — Build engine vs playbook (mechanism/policy separation)
- ADR-0007 — Cloud sandbox as a sibling track, not an installer mode
- ADR-0008 — Engine language: Go (thin POSIX-sh bootstrap)
- ADR-0009 — Phase 1 dogfood stack is Go (Python/TS → Phase 2)
- ADR-0010 — Two logs: audit (what) + diagnostic (how)
- ADR-0011 — build reconciles by a provision sentinel (presence ≠ converged)
- ADR-0012 — Prebuilt base image bakes the toolchain (provision = thin overlay)
- ADR-0013 — Requirements traceability via a machine-readable FR registry
- ADR-0014 — Agent provisioning + in-container credential login
- ADR-0015 — Harness home: config materializes into the agent volume; state accretes in it
- ADR-0016 — Entry verbs: exec-first passthrough; the audit log is the structured surface
- ADR-0017 — Writer remote-trust split
- ADR-0018 — Harness trust/opt-in flags are declared playbook config, materialized at build
- ADR-0019 — Container runs as a configurable, default-root non-root user
- ADR-0020 — Autonomy closed-loop (PARK / pull-next / re-surface drain)
- ADR-0021 — Role resolution under multi-role-per-container (Proposed)
- ADR-0022 — Autonomy substrate: ephemeral worker + backlog-ready pull (Proposed)
- ADR-0026 — VCS credential volume: sibling `~/.config/gh` volume + gh credential helper (Accepted)

## Template

```
# ADR-NNNN — <title>
**Date:** YYYY-MM-DD   **Status:** Accepted | Superseded by ADR-XXXX | Deprecated
## Context
## Decision
## Alternatives considered
## Consequences (positive / trade-offs / revisit-if)
```
