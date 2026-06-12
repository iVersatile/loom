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

Requirements: Docker, plus a Go toolchain (≥ 1.26) **or** a prebuilt `loom`
binary. There is no release channel yet (Phase 1), so on a no-Go machine the
binary comes from a machine that has Go: `go build -o loom ./cmd/loom` there
(cross-compile with `GOOS`/`GOARCH` as needed), then copy it into this clone
as `bin/loom` — `mkdir -p bin` first, a fresh clone has no `bin/` — or point
`LOOM_BIN` at it (bootstrap entry contract, docs/SPEC-verbs.md).

```sh
sh bootstrap/loom-bootstrap.sh build     # builds bin/loom if absent, then: one
                                         # unattended build → container + lock
bin/loom exec -- go version              # run a command inside it (login env,
                                         # project mounted at /workspace/<name>)
bin/loom plan                            # diff reality vs the playbook (--json on
                                         # every state verb)
```

On a machine that may already host loom environments, run `bin/loom plan`
before `build`: it grades the container against the playbook without mutating
anything (exit 2 = work outstanding, 0 = converged), and its verdict names the
target container.

The container is named from the playbook `name:`, so it's **one instance per
project per host** — a second checkout of the same project collides with the
first; edit `name:` in the second checkout's `loom.yml` to run them side by
side.

The first `build` provisions live from `debian:bookworm-slim` (apt + Go
toolchain installs) — expect minutes, not seconds, until the prebuilt base
image lands (docs/PLAN.md Phase 2, ADR-0012). A host with the base image
already cached (say, one that runs another loom container) builds much faster
than a truly cold machine — don't let a warm host set your expectations.

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
