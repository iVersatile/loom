# ADR-0013 — Requirements traceability via a machine-readable FR registry
**Date:** 2026-06-09   **Status:** Accepted

## Context
Autonomy needs a machine-checkable definition of *done*: an agent cannot
self-verify a phase against prose exit criteria, and "the tests pass" does not
prove every required behavior is tested. Today coverage is a hand-kept table in
TESTING.md that can drift silently — nothing proves each required behavior has a
test, nor that a test still maps to a live spec clause. We need traceability that
is grounded in the specs rather than a parallel source of truth.

## Decision
A **functional-requirement (FR) registry**: a structured, machine-readable
projection of the specs into atomic, testable assertions, each traced to its
source spec clause and to ≥1 passing test.

**Registry.** FRs live in a sibling artifact, **never in the playbook** — the
playbook describes the *environment*; the registry describes Loom's required
*behavior*; conflating them overloads the playbook. The registry is
machine-readable; every FR has a stable ID and cites its source spec section.

**Registry location & format.** A single `docs/FR-registry.yml`, sibling to the
`SPEC-*.md` contracts, YAML-authored; each entry carries `id`, `kind`,
`statement`, `source`, `tests`, `status`. `kind` (`behavioral | invariant |
guardrail | schema`) selects the coverage rule `verify` applies. Split into
`docs/FR/<area>.yml` only when the single file becomes unwieldy.

**Source of truth — the arrow is `spec → FR → test`.** The specs are
authoritative: SPEC-verbs, SPEC-playbook, and the AI-first invariants (ADR-0005 /
RULES §5). The registry is a *projection* of them, not a peer: each FR is an
atomic, testable assertion extracted from a spec clause. FRs are **authored**
(human, or agent-with-review), never auto-generated from spec text, and the
registry never originates a requirement not grounded in a spec clause.

**Granularity — one FR = one independently-verifiable behavior that can fail on
its own.**
- **Behavioral** — one per testable spec clause; the spec's structure sets the
  grain. Rationale, examples, and cross-references yield no FRs (expect *fewer*
  FRs than spec bullets).
- **AI-first invariants** — *global*: one FR asserted once (e.g. `FR-INV-001`:
  all mutating verbs are idempotent), satisfied by a single parametrized suite
  across all applicable verbs. The link is one invariant FR ← one parametrized
  suite ← N per-verb cases. A parametrized suite **may** satisfy one FR — do not
  require N separate test files, and do not split an invariant into
  one-FR-per-verb.
- **Specific guardrails** — *per-hook, not per-pattern*: one FR per guardrail hook
  (e.g. `FR-GUARD-BASH`, `FR-GUARD-BRANCH`, `FR-GUARD-PROTECT-PATHS`). The
  statement is the hook's guarantee ("blocks its defined class of dangerous
  actions"); its test is the **full pattern suite** for that hook — covering every
  pattern the hook enforces, not a single example.
- **Schema / resolution** — one per merge/resolution rule (each fails
  independently; e.g. layer order, `rules:` union, `dotfiles:` later-wins).
- **Consistency heuristic** — if you can't write one clear pass/fail test for an
  FR, it's too coarse — split it; if two FRs would never appear separately in a
  failure report, it's too fine — merge them.

**Enforcement — tiered**, mirroring the gate model (RULES §7: fast/advisory local,
authoritative at PR/release) and ADR-0005 (guardrails by mechanism, not trust).
- **Advisory by default** — reports orphan tests, uncovered FRs, and FRs whose
  source spec clause no longer exists.
- **Blocking only at boundaries** — phase completion, merge to main, release;
  never per-commit or mid-exploration.
- **Generated and enforced, not hand-maintained** — a `verify` check computes
  integrity; CI fails on violations at the blocking boundaries. Hand-maintenance
  is rejected as a rot-then-lie failure mode (a green matrix that no longer
  reflects reality is worse than none).

**What `verify` checks — both joints of the chain.**
- **FR → test:** every FR has ≥1 passing linked test; for invariant FRs, a passing
  parametrized suite covering all applicable verbs; for guardrail FRs, a passing
  suite covering all of that hook's patterns. Every test cites a valid FR.
- **FR → spec:** every FR cites an existing spec section; flag FRs whose source
  clause was removed (drift detection *toward* the specs).

**Timing.** FRs accrete *during* a phase, seeded from the verb contracts in
SPEC-verbs (each testable clause → behavioral FR; the invariants → the global FR
set; the guardrail hooks → one FR each) — not created retroactively at phase end.
**Bootstrap exception:** Phase 1 shipped before this registry existed, so its FRs
are back-filled once, from the now-frozen Phase 1 contracts and the tests that
already pass. This is a one-time seeding, not a reversal of the during-the-phase
rule, which governs Phase 2 onward.

## Alternatives considered
- **FRs in the playbook** — overloads it; the playbook is environment
  desired-state, not Loom's behavior spec. Rejected.
- **Hand-maintained registry** — rots, then lies. Rejected for generated +
  enforced.
- **Auto-generated FRs from spec text** — brittle parser coupling to prose;
  author with traceability instead. Rejected.
- **Per-verb FRs** — too coarse; a failure does not localize. Rejected.
- **One-invariant-FR-per-verb** — FR explosion for what is a single guarantee.
  Rejected for the global invariant FR + parametrized suite.
- **Per-pattern guardrail FRs** — heavier than warranted now; a hook's patterns
  are co-maintained per hook. Rejected now (see revisit-if).
- **No traceability** — leaves "done" undefined for an autonomous agent. Rejected
  (this is the whole point).

## Consequences
- Positive: an autonomous agent can self-verify "done" against grounded
  requirements; failures localize at the right grain; drift is caught in *both*
  directions (orphan tests and dead spec clauses).
- Trade-offs: a `verify` FR-check to build and maintain; authoring discipline to
  keep FR↔spec citations current; per-hook guardrail FRs mean a hook's test must
  exercise its whole pattern set or the coverage is illusory.
- Revisit if: a guardrail hook's patterns begin regressing independently → split
  that hook into per-pattern FRs; or an invariant suite accrues cases that need
  independent failure reporting → reconsider the one-FR-per-invariant grain.
