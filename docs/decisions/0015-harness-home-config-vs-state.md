# ADR-0015 — Harness home: declared config materializes into the agent volume; mutable state accretes in it
**Date:** 2026-06-10   **Status:** Accepted (2026-06-10 — human acceptance via PR #24 merge; drafted by agent per RULES §5/C3)

## Context
The first loom-built dev container (`loom-dev`, T12 cutover) materialized a
statusline-only `~/.claude`: no hooks/guards, no skills, no permissions, no
memory, no git identity (thread **T16**). Everything the dev experience depends
on beyond `settings.json` was hand-made — exactly the drift ADR-0002/0006 exist
to prevent. This is T12's usability criterion 4, the last dogfood blocker
besides cutover bookkeeping.

Dogfooding inside `loom-dev` (2026-06-10, threads T18/T19) produced live
evidence that shapes the answer:
- A **converge build re-materialized `~/.claude/settings.json` and erased
  harness-written runtime state** (a permission-mode preference the harness had
  saved into the same file). Declared-config-wins is correct — but it proves
  one file cannot serve as both declared config and harness-mutable state.
- **Repo-tracked `.claude/`** (permissions allowlist, agent defs — PR #20)
  works with *zero engine involvement*: the working tree is the mount, so it
  survives rebuilds for free. But it only covers opted-in projects, post-clone,
  and cannot carry base-wide policy or identity.
- The harness itself created **mutable memory** under `~/.claude/projects/`
  inside the T14 volume — state loom never declared, surviving rebuilds by
  riding the volume. The claims script flags it SURPRISE; it should be design.
- **Session restarts are a certainty** (rebuild, crash, model switch); regaining
  context is currently a hand-carried prompt (`.scratch/` handoff convention).

## Decision
**The boundary cuts at mutability, at the `~/.claude` (agent-home volume) seam.**

1. **Declared config materializes, every build.** Harness config — hooks
   (guard-bash, branch-guard, protect-paths, session snapshot/orient), skills,
   user-level permissions/`settings.json`, statusline, git identity
   (`~/.gitconfig`) — is declared in the playbook, sourced from the config
   source, and materialized through the existing dotfiles pipeline (staging →
   `docker cp` through the volume mount) on every build. Versioned, reviewable,
   re-converged; drift is erased by mechanism.
2. **Mutable state accretes in the volume and is never materialized:**
   credentials (ADR-0014 addendum), harness memory (`projects/`), session
   history, and harness-runtime preferences. `settings.json` is config;
   `settings.local.json` is state — the harness writes there, loom never
   touches it. Never bind-shared with the host or another container (the
   session-journal corruption risk).
3. **Schema: a `harness:` section, explicit-by-reference like `rules:`,** not a
   generalization of `dotfiles:`. Harness artifacts have semantics plain
   dotfiles lack: hook registration inside `settings.json`, per-agent
   namespacing (claude today, others later), policy review weight (a
   permissions file is a guardrail, ADR-0005). Phase 1 keeps the resolution
   rule simple: `settings.json` is base-authored whole-file (carrying its hook
   registrations with it); per-tier key-merge stays deferred (SPEC-playbook).
4. **Two-tier policy split (ADR-0004), confirmed by T18:** base playbook owns
   env-wide harness config (guards, identity, base deny-rules); the **project
   repo owns per-project policy** (`.claude/settings.json` allowlist, agent
   defs) with no engine involvement — loom may later *assert* its presence
   (`doctor`/`verify`), never author it.
5. **Session continuity is harness-home config, not a convention:** a
   session-end snapshot hook + session-start orientation are first-class
   `harness:` items, replacing the hand-carried `.scratch/` prompt.
6. **Memory seeds empty.** Continuity comes from repo docs + the snapshot hook,
   not from importing another machine's memory (single-writer; clean cut).
7. **T10 rule:** every materialization targets the parameterized `$HOME`, never
   a literal `/root`.

## Alternatives considered
- **Bind-mount the host `~/.claude`.** Rejected: shadows materialized config,
  shares mutable state across concurrent writers, and is dead on macOS for
  creds (ADR-0014).
- **Copy-once at container create.** Rejected: config drifts from source —
  T7's bug class as a design.
- **Generalize `dotfiles:` instead of `harness:`.** Rejected (narrowly): the
  pipeline is reused either way, but a dedicated section keeps guardrail-weight
  artifacts explicit and per-agent semantics expressible. Revisit if `harness:`
  turns out to be pure sugar over dotfile entries.
- **Repo-tracked `.claude/` as the general answer.** Rejected for base-wide
  config (coverage gap, no identity, no pre-clone existence); adopted for the
  per-project policy slice where it is strictly better (T18 evidence).

## Consequences
- Positive: rebuild ⇒ converged harness home; T12 criterion 4 becomes
  engine-guaranteed and claims-checkable (the T16 GAP list in
  `verify-loom-dev.sh` shrinks to zero and flips to PRESERVE claims).
- **Precondition: T7 must be fixed first.** Home re-sync is gated on the
  tools/agents provision digest, so harness-config-only changes would not
  converge. Fold home-staging content into the change trigger before or with
  the first `harness:` materialization.
- Engine work that follows (separate PRs, FRs per behavior): `harness:` schema
  + materialize handlers (+ executable bits beyond `.sh`), settings/hook
  registration, `doctor` checks promoted from the claims script (T17 path).
- The memory SURPRISE becomes designed behavior; update the claims script
  expectations when the schema lands.
- Out of scope, recorded: non-`~/.claude` agent state (e.g. `~/.config/gh`,
  VCS credentials — T18's push gap) needs the volume model widened or a
  sibling volume; that belongs to the T15-successor credential ADR.
- Revisit if: multi-agent homes (gemini/codex) need per-agent volumes, or
  key-merge lands in SPEC-playbook and per-project `settings.json` overlays
  become safe.
