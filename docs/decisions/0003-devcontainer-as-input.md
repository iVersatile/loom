# ADR-0003 — Devcontainer as input (import-and-enrich), not output
**Date:** 2026-06-05   **Status:** Accepted

## Context
Devcontainer-compatibility is desirable for ecosystem reach (VS Code, Codespaces,
corporate adoption), but reshaping Loom's schema to *be* devcontainer would lose
the policy/intent/AI-context that is Loom's value. The user explicitly rejected
that trade-off while recognizing the need for compatibility.

## Decision
Treat `devcontainer.json` as an **input format Loom imports and enriches**, not an
output format Loom degrades to. Staged compatibility:
- Stage 1: import a base devcontainer; run as a plain devcontainer (compatible).
- Stage 2: layer the two-tier env-wide config, hooks, rules.
- Stage 3: layer the AI-agent operating context.
A deterministic **export** (Loom → devcontainer) is a later, lossy interop nicety;
**import** (devcontainer → Loom) may use an AI skill for intent inference.

## Alternatives considered
- **Export-first / devcontainer as output** — makes devcontainer the ceiling;
  forces schema compromise. Rejected.
- **Native devcontainer schema** — inherits ecosystem free but can't model policy.
  Rejected as source-of-truth.

## Consequences
- Positive: meet the ecosystem where it is; devcontainer's gaps (policy, two-tier,
  AI-context) become Loom's value-add layer; no schema compromise.
- Trade-offs: must maintain an importer; export is lossy (policy/intent don't map).
- Revisit if: the devcontainer spec grows native policy/multi-tier concepts.
