# Loom

Loom builds and maintains **two-tier, AI-first development environments** from a
versioned **playbook**: a shared base layer plus per-project overlays, one
container per project, devcontainer-compatible at the floor — designed to be
operated by a human or an autonomous AI coding agent.

> Status: **Phase 0 — seed.** Specs and decisions are being frozen before engine
> code (see docs/PLAN.md). Nothing is built yet.

## Start here
- **docs/CHARTER.md** — what Loom is, goals, the AI-first north star, non-goals.
- **docs/decisions/** — the architectural decisions and their rationale.
- **docs/SPEC-playbook.md** / **docs/SPEC-verbs.md** — the contracts.
- **docs/PLAN.md** — roadmap and current phase.
- **docs/RULES.md** — how we work here.
- **AGENTS.md** — entry point for AI contributors.

## Shape (planned)
```
loom <verb>:  detect | plan | build | update | teardown | import | doctor
playbook:     base (env-wide)  +  stack/<lang>  +  overlay/<project>
isolation:    one container per project, shared base image
```
