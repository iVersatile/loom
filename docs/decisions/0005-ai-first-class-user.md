# ADR-0005 — AI agent as first-class user
**Date:** 2026-06-05   **Status:** Accepted

## Context
Loom's north star is environments operable by an autonomous AI agent with no
human in the loop. This is a design constraint, not a feature.

## Decision
AI is first-class for both the tooling and the environment. Hard requirements on
every verb and environment:
- Structured `--json` output alongside human output.
- Idempotent, declarative reconcile (re-runnable, recoverable).
- Self-description (agent queries what/why/rules/state from files).
- Guardrails enforced by mechanism (hooks, deny-lists, gates), not trust —
  including guards on the playbook itself (an agent must not silently weaken a
  deny-rule or add an exfiltrating tool).
- Auditable trail for every detect/plan/change.
Design test: "could an agent do this unattended, and would the guardrails hold if
it tried the worst thing?"

## Alternatives considered
- **Human-only, AI as afterthought** — would bolt on JSON later and under-build
  guardrails. Rejected given the north star.

## Consequences
- Positive: autonomy-ready; guardrails double as human safety; auditability.
- Trade-offs: dual-render every command; more upfront guard engineering.
- Revisit if: never — this is the north star.
