# Phase-close review gate — rubric (P7, human-decided 2026-06-11)

Every phase closes through a mandatory independent review BEFORE the human's
close edit. Decided by the human 2026-06-11 (draft 016, three counters
confirmed); first executed 2026-06-12 for Phase 1 —
`docs/reviews/phase-1-review.md` is the precedent and the format template.

## Scope — three dimensions, each its own verdict

| Dimension | Grades |
|---|---|
| **Security** | guardrail wiring (declared → materialized → enforced), credential reach, egress posture, audit integrity |
| **Architecture** | contract honesty (declared flags/tiers act or error), verdict/action domain agreement, layering, spec↔code joints |
| **Harness-health** | hooks present AND live, permission-stack coherence, transport hygiene (inbox/drain/flips.log), skills state |

## Independence rule

The reviewer is an independence-ruled **hat, not a member**: run by a context
that **authored nothing in the phase** — excludes the Writer AND the advisor's
authoring contexts. Acceptable vehicles: fresh sessions, adversarial
subagents, cloud runs. One fresh context per dimension (Phase 1 used three,
spawned by the advisor).

## Severity rubric (defined before the review runs, never after)

| Severity | Consequence |
|---|---|
| **Critical** | NO waiver — fix lands or the phase stays open. Re-scoping a Critical out of the phase is a human ruling, never an agent call. |
| **High** | fix, OR the human accepts the risk **in writing**. No written acceptance ⇒ defaults to fix. |
| **Medium / Low** | → backlog, triaged like any other work. |

## Conduct rules

- **Findings name the WHAT, never the HOW** — the fix approach is the
  Writer's freedom (human's standing rule: leave room for agent decisions).
- **Judges, then doctor inherits**: whatever a reviewer hand-checks once,
  doctor mechanizes next — each review's "mechanization note" feeds the
  doctor checklist. (Phase-1 yield: guardrails *wired* not just present →
  FR-DOCTOR-003; TAKEN-claim liveness and snapshot freshness → backlog.)
- The review runs **before** the close edit; verdicts and rulings are
  recorded in `docs/reviews/phase-N-review.md` in the same document.
- No agent self-approves a phase completion (AGENTS.md boundary) — the gate
  produces evidence; the close stays human.

## RULES pointer (prepared diff — frozen path, human admin-merge)

`docs/RULES.md` is frozen; the pointer below is PREPARED here for the human
to apply (admin-merge = acceptance). Proposed addition to **§7 Gates**:

```diff
 ## §7 Gates
 - Lint/type/test/secret-scan run as local pre-commit; adversarial review + heavy
   tests + security run in CI (advisory review, non-blocking). Don't bypass.
+- Phases close through the independent review gate (docs/review-gate.md):
+  Critical = no waiver; High = fix or written risk acceptance; reviewer
+  authored nothing in the phase.
```
