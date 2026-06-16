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
  *Phase status (amended 2026-06-10, T5):* `resolved` is probed **inside the
  built container** (the lock pins the container's reality, never the build
  host's) and the base-image digest is produced; the **per-tool `digest`
  producer is Phase 2** — the field is in the schema but not yet populated.

## Principles

- **Thin surface, pinned underneath.** Authors write intent; the resolver + lock
  capture exact versions. Never require authors to write exact pins (but allow it).
- **Declarative desired-state.** The file describes the *end state*; the engine
  computes the diff. A tool's *absence* is meaningful (→ remove it).
- **Self-describing for AI.** The playbook references the rules/skills/intent an
  AI needs, not just the tools (ADR-0005). This includes `dotfiles:` — a playbook
  reader can see that a custom shell prompt or statusline exists, not just infer it.
- **Two scopes.** A machine/base playbook + per-project overlay playbooks, merged
  at build time (later layer wins for whole files; lists concatenate).
- **Declared `$HOME`, not hand-edited.** Shell/agent dotfiles (prompt, statusline)
  are referenced via `dotfiles:` and materialized into the container `$HOME` by
  `build`. Editing them inside a running container is ephemeral — they survive a
  rebuild *because* they are reconciled from the config source every build, never
  applied ad-hoc (ADR-0001, ADR-0006). Env-wide → base tier; per-project → overlay.

## Base playbook (machine/env-wide tier)

`~/.loom/playbook.yml` (illustrative)
```yaml
loom: 1                      # schema version
tier: base

# user: dev                  # optional: env-wide non-root runtime user (T10).
                             # Unset = root (compatibility). A project overlay
                             # may override. See "user: field" below.

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

dotfiles:                    # references, not inline bodies — materialized into $HOME
  - claude/settings.json     # env-wide ~/.claude config (e.g. statusline)
  - claude/statusline.sh     # the statusline command script

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

dotfiles:                    # per-project $HOME config (e.g. a project bash prompt)
  - bash/prompt.py.sh        # overlay tier wins over base for the same target path

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
Example: a go project resolves `base + stacks/go + overlays/loom` (the Phase 1
dogfood path, ADR-0009). A python project resolves `base + stacks/python +
overlays/<project>`. No cross-contamination.

**`dotfiles:` resolution.** `dotfiles:` entries are reference names resolved against
the config source (like `rules:`/`hooks:`), each mapping to a target path under
`$HOME`. They follow the same layer order: a later tier referencing the same target
path replaces the earlier file (whole-file, later-wins). One exception is called out
below for `~/.claude/settings.json`.

**`~/.claude/settings.json` merge (Phase 1).** This is a single JSON file, so the
whole-file/later-wins rule means a base and an overlay `settings.json` would *clobber*
rather than key-merge. For Phase 1 the statusline lives in the **base** tier only and
no project overrides it; key-level JSON merge for `settings.json` is **deferred** (see
Open questions and ADR-0004 "revisit if").

## Shell config model (T4, decided 2026-06-11)

One dotfile directory — `~/.bashrc.d/` — owns shell configuration for **all**
shell types: the engine wires both login (`.profile`) and interactive
(`.bashrc`) init files to source every `~/.bashrc.d/*.sh`, unconditionally
(not gated on the tool set). A `bash/*` dotfile therefore applies however the
shell is invoked, including the non-interactive login shells `exec`/`shell`
use.

**PATH is dotfile-owned.** Stack and agent PATH entries the engine knows about
(the Go toolchain's `$HOME/go/bin`, an agent's `$HOME/.local/bin`) are
**engine-generated dotfiles** in the same directory (`path.go.sh`,
`path.local.sh`) — staged, drift-graded, and audited exactly like declared
dotfiles; they emit `$HOME`, never a hardcoded user home. A project needing
more PATH ships its own `bash/path.*.sh` dotfile.

**There is deliberately NO `path:` field** in the schema. Rejected for
Phase 1; reopen only if a second stack needs PATH-ordering semantics
(ADR-0004 "revisit if" class).

## `user:` field (added 2026-06-13, T10 / ADR-0019)

**`user:` (optional, scalar, later-wins): the container's runtime user. Unset
means root (compatibility). A non-root user is created at provision (non-root;
uid 1000 by default, system-assigned on collision; doctor verifies by name),
home `/home/<user>`; every materialization targets the resolved `$HOME`
(ADR-0015 T10 rule); entry verbs run as this user (ADR-0016 decision 7).**

Authored at any tier (last-non-empty-wins, like other scalars — *not* base-only
like `harness.settings:`); the expected shape is an env-wide base default with a
per-project override. `$HOME` derives from the value: `root → /root`, any other
`<user> → /home/<user>`. `user: root` is permitted and means the default — the
"a later layer re-grants root" edge is enforced at the full-auto re-evaluation
gate, not special-cased in the scalar merge (ADR-0019). The engine behavior this
clause authorizes (provision-as-root / run-as-user split, `useradd`, ownership
`chown`, doctor `container:user`, the role marker that replaces the uid guess)
is sliced across T10 PR 3–4; this clause + the schema/merge/validate +
`ContainerSpec.User/Home` plumbing is PR 2.

## `role:` field (added 2026-06-15, T10 / ADR-0019 PR4 §5, LL-014)

**`role:` (optional, scalar, later-wins): the container's loom-role identity, the
declarative in-tree source for the root-owned `/var/lib/loom/role` marker `loom
build` writes.** Unset means no marker (a root build is byte-identical to
pre-PR4). A non-root `user:` with an empty/invalid `role:` is a **build-time
error** — that combination silently breaks the drain role-guard (the marker is
how a non-root agent's role is resolved without trusting `id -un`).

Authored at any tier (last-non-empty-wins, like `user:`); the same single-token
shape (no whitespace, no `/` or `:`), with the engine additionally enforcing a
marker-safe charset (`[A-Za-z0-9_-]`) at write time — anything else writes no
marker (fail-safe). The marker is produced **the loom way**: by `loom build` from
this tree-recorded value, identically on every host, folded into the convergence
digest so a missing marker self-heals on the next plain build (mirrors the
`/var/lib/loom/home` sentinel) — never hand-written into a container (LL-014).

The ambient `LOOM_SESSION_ROLE` env that PR4 Part 1 first used is **demoted** to
an explicit override / test-seam (it wins over `role:` when set, so a second seat
sharing one tree — e.g. the advisor — can override without editing the playbook).
Conceptual home of the role model is ADR-0021. Whether anything *reads* the
marker (the drain role-guard swap) is PR4 Part 2, human-applied.

## `harness:` section (added 2026-06-11, ADR-0015)

Harness-home config — artifacts with semantics plain dotfiles lack: hook
registration inside `settings.json`, executable bits, per-agent namespacing
(claude today, others later), guardrail policy weight (ADR-0005). Entries are
**explicit-by-reference** like `rules:`, resolved against the config source,
and materialized into the agent home (`~/.claude` for claude-code — always
the parameterized `$HOME`, never a literal path) on every build through the
same staging pipeline as `dotfiles:` (write-if-changed; the home-digest
sentinel covers the staged tree, T7).

```yaml
harness:
  claude:                           # per-agent namespace
    settings: claude/settings.json  # resolves from dotfiles/; base-authored
                                    #   WHOLE FILE (Phase 1) — it carries its
                                    #   own hook registrations
    trust: claude/trust.json        # resolves from dotfiles/; whole-file to
                                    #   $HOME/.<agent>.json (SIBLING of the
                                    #   agent home) — trust/opt-in flags
                                    #   (added 2026-06-12, 036 ruling, ADR-0018)
    hooks:                          # resolve from hooks/<name>; materialize to
      - guard-bash                  #   ~/.claude/hooks/<name>, executable
      - session-snapshot
    skills:                         # resolve from skills/<name>/; materialize
      - replan                      #   to ~/.claude/skills/<name>/ (dir copy)
```

- Layer resolution follows `dotfiles:`: base → stack → overlay, whole-file
  later-wins per target path. Phase 1: `settings.json` is base-tier only,
  no key-merge (Open question 1); the engine does not synthesize hook
  registrations — the declared `settings.json` carries them (ADR-0015
  decision 3).
- The top-level `hooks:` field is unchanged (guardrail references `doctor`
  verifies in the config source); `harness.<agent>.hooks` selects which of
  them materialize into that agent's home.
- Git identity (`~/.gitconfig`) is harness config by weight but plain by
  shape — it ships as a `dotfiles:` reference, no new field.
- `trust:` (added 2026-06-12 — 036 ruling "2-plus", ADR-0018) is a dotfiles/
  reference materialized WHOLE-FILE to `$HOME/.<agent>.json`, the harness's
  top-level state file and a *sibling* of the agent home. It carries the
  trust/opt-in flags the harness reads at session start
  (`hasTrustDialogAccepted` et al.). That file lives on the container
  overlay, not the agent-home volume — it dies on recreate; build
  re-materializing it is what makes the opt-in durable
  (declare-or-rederive, ADR-0014 precedent). Layer rule: last non-empty
  wins; trust posture is project-tier by doctrine (the flags name project
  paths). Trust changes ship as playbook edits — a flip is a PR, and the
  T23 flips.log records what happened outside that flow.
- Mutable state (`settings.local.json`, `projects/`, credentials, session
  history) is never a valid `harness:` target: the engine materializes
  declared config only and never touches state (ADR-0015 decision 2).
  `trust:` is the one deliberate carve-out (036 ruling): the *declared*
  flag file is config the playbook owns — the engine still writes only
  declared content at its declared target, never reads or key-merges the
  live file. Whatever else the live `$HOME/.<agent>.json` accumulates at
  runtime is rederivable cache: a home re-sync overwrites it, and that is
  accepted by ruling ("caches may die").

## Devcontainer compatibility (ADR-0003)

The playbook is a **superset** of what devcontainer models. Import maps a
`devcontainer.json` into the project tier (image/features/ports/env/commands →
playbook fields); the two-tier base, rules, hooks, and AI-context are *added* on
top (Loom's value layer). Export (later, lossy) emits a `devcontainer.json` from
the environment fields only; policy/intent do not map and stay in repo docs.

## Frozen decisions (2026-06-08, addendum)

- **`rules:` resolution (was open Q1-remaining).** Frozen: **explicit-by-reference**.
  A `stack:` contributes *default* rule references, and tiers may add more; the
  resolved set is the union, deduplicated, in layer order. Authors always see the
  references they get, rather than rules appearing by stack magic. (Phase 1 parser.)
- **`dotfiles:` field.** Frozen for Phase 1: a reference list at both tiers,
  materialized into `$HOME` at build with whole-file/later-wins layering.

## Open questions

Format (Q1), lockfile granularity (Q3), `rules:` resolution, and the `dotfiles:`
field are frozen (see *Frozen decisions*). Remaining:

1. Key-level merge for `~/.claude/settings.json` (so base + overlay can both set keys
   instead of clobbering). Deferred past Phase 1; Phase 1 is base-only whole-file.
2. Where the env-wide base playbook lives when the cloud track is in play
   (ADR-0007): on the VM, in a volume, or fetched from the config source.
   (Phase 5; not blocking.)
