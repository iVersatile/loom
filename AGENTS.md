# AGENTS.md — Loom

Canonical instructions for agent harnesses working ON Loom itself. CLAUDE.md and
GEMINI.md symlink here. Thin pointer — detail lives in docs/.

## Orient yourself first
1. docs/CHARTER.md — what Loom is, goals, north star, non-goals.
2. docs/decisions/ — why the architecture is the way it is (ADR-0001..0008).
3. docs/SPEC-playbook.md, docs/SPEC-verbs.md — the contracts you implement against.
4. docs/RULES.md — how we work here (source of truth).
5. docs/PLAN.md — current phase and exit criteria.

## The one rule that overrides convenience
Specs before code (RULES §2, ADR-0006). If implementation and a frozen spec
disagree, the code is wrong. Propose a spec change via PR + ADR; don't drift.

## AI-first invariants (RULES §5) — you must uphold these
- Human + `--json` on every verb. Idempotent, recoverable, audit-logged mutations.
- Guardrails are mechanism, not trust. You may not weaken a deny-rule or add a
  tool that exfiltrates. The design test: would the guardrails hold if you tried
  the worst thing?

## Boundaries
- No commit on main; no secrets in code/logs; no --no-verify/sudo/force-push.
- Run the gate before commit; never self-approve a release or a phase completion.
