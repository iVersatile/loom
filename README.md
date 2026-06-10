# Loom

Loom builds and maintains **two-tier, AI-first development environments** from a
versioned **playbook**: a shared base layer plus per-project overlays, one
container per project, devcontainer-compatible at the floor — designed to be
operated by a human or an autonomous AI coding agent.

> Status: **Phase 1 — spine.** `detect / plan / build / exec / teardown` run
> end-to-end for the Go dogfood — loom builds and converges its own dev
> container, and is developed from inside it, through its own `exec` door.
> Specs and decisions freeze before code (see docs/PLAN.md for phases and the
> live tactical queue).

## Quickstart

Requirements: a Go toolchain (≥ 1.26) and Docker.

```sh
sh bootstrap/loom-bootstrap.sh build     # builds bin/loom if absent, then: one
                                         # unattended build → container + lock
bin/loom exec -- go version              # run a command inside it (login env,
                                         # project mounted at /workspace/<name>)
bin/loom plan                            # diff reality vs the playbook (--json on
                                         # every state verb)
```

The first `build` provisions live from `debian:bookworm-slim` (apt + Go
toolchain installs) — expect minutes, not seconds, until the prebuilt base
image lands (docs/PLAN.md Phase 2, ADR-0012).

## Start here
- **docs/CHARTER.md** — what Loom is, goals, the AI-first north star, non-goals.
- **docs/decisions/** — the architectural decisions and their rationale.
- **docs/SPEC-playbook.md** / **docs/SPEC-verbs.md** — the contracts.
- **docs/PLAN.md** — roadmap, current phase, and the live tactical queue.
- **docs/RULES.md** — how we work here.
- **docs/TEAM.md** — roles, write discipline, merge policy.
- **docs/TOPOLOGY.md** — where loom runs and who operates it.
- **AGENTS.md** — entry point for AI contributors.

## Shape
```
loom <verb>:  detect | plan | build | exec | teardown | doctor      (built)
              shell (spec'd, staged) | update | import | export     (ahead)
playbook:     base (env-wide)  +  stack/<lang>  +  overlay/<project>
isolation:    one container per project, shared base image
```
