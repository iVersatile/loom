# Rule: loom/project

Loom's own project rules (overlay tier). Source of truth is docs/RULES.md; this
is the playbook-referenced pointer.

- Specs before code (RULES §2); frozen contracts change via PR + ADR.
- Every verb: human + `--json`; mutations idempotent, recoverable, audit-logged.
- Run the gate before commit; never self-approve a release or phase completion.
