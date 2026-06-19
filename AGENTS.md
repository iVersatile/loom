# AGENTS.md — Loom

Canonical instructions for agent harnesses working ON Loom itself. CLAUDE.md and
GEMINI.md symlink here. Thin pointer — detail lives in docs/.

## Orient yourself first
1. docs/CHARTER.md — what Loom is, goals, north star, non-goals.
2. docs/WORKSTREAMS.md — the project-level map (which arc am I in): The Spine ·
   AI-First · The Run · Target Env · Guardrails · Verification.
3. docs/decisions/ — why the architecture is the way it is (ADR-0001..0010).
4. docs/SPEC-playbook.md, docs/SPEC-verbs.md — the contracts you implement against.
5. docs/RULES.md — how we work here (source of truth).
6. docs/PLAN.md — current phase and exit criteria (+ the tactical queue).
7. docs/TOPOLOGY.md — where loom runs and who operates it: mac-dev-topology
   (validated), windows-dev-topology (declared), ai-user-topology (north star).
8. docs/HARNESS.md — how the agent harness is wired: the config/state seam,
   permission stack, convergence sentinels (synthesis; sources win on conflict).

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
- Conventional Commits; **never add a `Co-Authored-By:` / Anthropic trailer** to commits.
- Run the gate before commit; never self-approve a release or a phase completion.

## Communication (replies to the human)
- Brief by default; details on demand. Lead with the answer.
- Prefer lists, tables, and diagrams over prose paragraphs.
- Present options (with a lean) when a decision has more than one shape.
- End every substantive reply with **Human action** (if any) + a 1–2 line summary.
