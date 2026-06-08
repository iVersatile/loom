# SPEC — Playbook Schema (draft)

> Status: draft for discussion. Examples are illustrative, not final. The schema
> is **thin** (intent by name@version), with a generated lockfile for exact pins
> (see ADR-0002). Two tiers (ADR-0004): a base playbook + per-project overlays.

## Principles

- **Thin surface, pinned underneath.** Authors write intent; the resolver + lock
  capture exact versions. Never require authors to write exact pins (but allow it).
- **Declarative desired-state.** The file describes the *end state*; the engine
  computes the diff. A tool's *absence* is meaningful (→ remove it).
- **Self-describing for AI.** The playbook references the rules/skills/intent an
  AI needs, not just the tools (ADR-0005).
- **Two scopes.** A machine/base playbook + per-project overlay playbooks, merged
  at build time (later layer wins for whole files; lists concatenate).

## Base playbook (machine/env-wide tier)

`~/.loom/playbook.yml` (illustrative)
```yaml
loom: 1                      # schema version
tier: base

# Agent harnesses available in every project's container
agents:
  - claude-code
  - codex
  - gemini

tools:                       # shared across all projects
  - jq
  - gitleaks
  - ripgrep
  - git

rules:                       # references, not inline bodies
  - common/safety            # resolves to a rules file in the config source
  # stack rules are added by overlays, not here

hooks:                       # env-wide guardrails (enforced by mechanism)
  - guard-bash
  - branch-guard
  - session-memory

config_source:               # where rules/skills/hooks resolve from (ADR-0006)
  type: local                # local | git
  path: ./config             # or: { type: git, ref: vX.Y.Z, url: ... }
```

## Project overlay playbook (per-project tier)

`<repo>/loom.yml` (illustrative)
```yaml
loom: 1
tier: project
name: prompiler

extends: base                # inherit the base tier

stack: python                # selects config/stacks/python (tools + rules)

tools:                       # project-specific, on top of stack defaults
  - python@3.11              # name@version — thin intent; lock pins exact
  - uv
  - ruff
  - mypy

overlay: prompiler           # most-specific layer (project rules/skills/CI)

rules:
  - python/strict            # stack rule
  # project source-of-truth rules live in docs/RULES.md, referenced by AGENTS.md

ports:
  - 8000

env:                         # names only — values come from .env / secret store
  - ANTHROPIC_API_KEY
  - OPENAI_API_KEY
  - GOOGLE_API_KEY

ci:                          # which CI templates to emit
  - ci
  - security
  - code-review
  - release
  - weekly-report
```

## Lockfile (generated, do not hand-edit)

`loom.lock` (illustrative) — the thick, exact realization of the thin playbook.
```yaml
loom_lock: 1
resolved_at: 2026-06-05T00:00:00Z
base_image: debian:bookworm-slim@sha256:...
tools:
  python: { intent: "3.11", resolved: "3.11.9", source: "brew", digest: "..." }
  ruff:   { intent: "latest", resolved: "0.5.2", source: "uv", digest: "..." }
agents:
  claude-code: { resolved: "x.y.z" }
```

## Layer resolution (ADR-0004)

Build-time merge order, later wins for whole files; lists concatenate:
```
base  →  stack/<lang>  →  overlay/<project>  →  user-local overrides
```
Example: a python project resolves `base + stacks/python + overlays/prompiler`.
A go project resolves `base + stacks/go` (no overlay). No cross-contamination.

## Devcontainer compatibility (ADR-0003)

The playbook is a **superset** of what devcontainer models. Import maps a
`devcontainer.json` into the project tier (image/features/ports/env/commands →
playbook fields); the two-tier base, rules, hooks, and AI-context are *added* on
top (Loom's value layer). Export (later, lossy) emits a `devcontainer.json` from
the environment fields only; policy/intent do not map and stay in repo docs.

## Open questions (resolve before freezing schema)

1. Playbook format: YAML (shown) vs TOML vs JSON. YAML is friendliest for humans
   and AI; JSON is easiest to emit/consume programmatically. Likely YAML authored,
   JSON also accepted.
2. How explicit should `rules:` references be vs auto-included by `stack:`?
3. Lockfile granularity — per-tool digest vs whole-image digest vs both.
4. Where the env-wide base playbook lives when the cloud track is in play
   (ADR-0007): on the VM, in a volume, or fetched from the config source.
