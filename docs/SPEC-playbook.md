# SPEC — Playbook Schema

> Status: **frozen for Phase 1** (reviewed 2026-06-08). Examples are illustrative;
> field shapes may still gain detail, but the decisions below are settled and code
> implements them (RULES §2). The schema is **thin** (intent by name@version), with
> a generated lockfile for exact pins (see ADR-0002). Two tiers (ADR-0004): a base
> playbook + per-project overlays.

## Frozen decisions (2026-06-08)

- **Format (was Q1).** YAML is the authored format; JSON is also accepted on
  **input** (parser reads either). Loom emits YAML. This keeps authoring friendly
  for humans and AI while letting programmatic producers hand Loom JSON directly.
- **Lockfile granularity (was Q3).** Record **both** per-tool (intent + resolved
  version + digest + source) **and** the base-image digest. Per-tool digests catch
  drift in individual installs; the image digest pins the floor. This is what
  reproducibility actually needs and mirrors Devbox's lock. The example below
  already reflects this shape.

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

## Open questions

Format (Q1) and lockfile granularity (Q3) are frozen — see *Frozen decisions*
above. Remaining:

1. How explicit should `rules:` references be vs auto-included by `stack:`?
   (Touches the Phase 1 parser; lean toward explicit-by-reference with `stack:`
   contributing defaults, but not yet frozen.)
2. Where the env-wide base playbook lives when the cloud track is in play
   (ADR-0007): on the VM, in a volume, or fetched from the config source.
   (Phase 5; not blocking.)
