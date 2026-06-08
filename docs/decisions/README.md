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

## Template

```
# ADR-NNNN — <title>
**Date:** YYYY-MM-DD   **Status:** Accepted | Superseded by ADR-XXXX | Deprecated
## Context
## Decision
## Alternatives considered
## Consequences (positive / trade-offs / revisit-if)
```
