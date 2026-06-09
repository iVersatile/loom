# ADR-0010 — Two logs: audit (what) + diagnostic (how)
**Date:** 2026-06-08   **Status:** Accepted

## Context
ADR-0005 / RULES §5 require an auditable trail for mutations; SPEC-verbs froze the
**action log** (`.loom/actions.log`, structured JSONL, one entry per mutation —
*what changed*). During Phase 1 build work, troubleshooting a container provision
needed the *raw* command output (docker calls, `set -x` trace) — a different
concern from audit. A diagnostic log was added in code first; this ADR records the
distinction and the decision to spec it (closing that specs-before-code gap,
RULES §2), and an autonomous agent debugging itself needs a predictable place to
find diagnostics (ADR-0005 self-description).

## Decision
Loom keeps **two distinct, per-project logs**, written by mutating verbs:

| | Audit log | Diagnostic log |
|---|---|---|
| Path | `.loom/actions.log` | `.loom/logs/<verb>.log` |
| Content | structured JSONL — *what changed* | raw command/trace output — *how it ran* |
| Audience | review, agents, compliance | troubleshooting |
| Stability | **format frozen** (a contract) | **format intentionally not frozen** |

The audit log is the governance contract (frozen shape). The diagnostic log is an
operability aid: its *existence, location, and per-mutating-verb production* are
the contract; its content/format is free-form and may change. Both are gitignored
runtime state.

## Alternatives considered
- **One log for both** — mixing structured audit with free-form trace makes the
  audit log unparseable and the trace un-greppable. Rejected.
- **Diagnostic log as undocumented impl detail** — it became a stable location the
  tests and users rely on; leaving it unspecced invites silent drift (the very
  thing RULES §2 guards). Rejected.
- **Freezing the diagnostic format too** — brittle; over-specs an impl detail
  against ADR-0002's thin-surface philosophy. Rejected.

## Consequences
- Positive: clean separation of governance vs operability; a predictable troubleshooting
  surface for humans and agents; the requirement becomes mechanism (a gate check
  asserts mutating verbs emit both) rather than trust.
- Trade-offs: every mutating verb must open/emit a diagnostic log; one more
  gitignored artifact.
- Revisit if: logs need rotation/retention policy, or a machine-wide aggregation
  (cloud track, ADR-0007).
