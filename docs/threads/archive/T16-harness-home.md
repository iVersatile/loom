# T16 — harness home: loom provides `settings.json` + statusline, not the rest   ✅ resolved (ADR-0015 Accepted)

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

**Status (2026-06-10):** promoted to **ADR-0015**, now **Accepted** (human
acceptance via PR #24 merge —
`docs/decisions/0015-harness-home-config-vs-state.md`). Remaining work is
implementation, tracked in the PLAN tactical queue ("T16 engine work"; T7
precondition fixed on `fix/t7-home-resync`). The ADR resolves this thread's
open questions:
config/state split at the volume seam, `harness:` section
(explicit-by-reference) over generalized `dotfiles:`, two-tier policy split
confirmed per T18, memory seeds empty, session continuity as declared hooks.
T7 is recorded as a precondition for the engine work. Entry kept below for the
full lean and the verification record.

Origin: the loom-dev verification pass (`.scratch/session-start-verification.md`)
— the predicted-LOSE list confirmed. The materialized `~/.claude` is statusline-
only; everything else the dev experience depends on is absent. This is T12's
usability criterion 4, the last unaddressed dogfood blocker besides cutover
itself.

**What's missing in `loom-dev` (confirmed absent).**
- **Hooks/guards:** SessionStart continuity snapshot, guard-bash, branch-guard,
  session-end — none run; `settings.json` is statusLine-only.
- **Memory:** `MEMORY.md` + auto-memories (`~/.claude/projects/<proj>/memory/`).
- **Skills / agents / plugins:** not materialized.
- **Permissions allow/deny:** no allowlist, so every session re-prompts.
- **Git identity:** `~/.gitconfig` neither mounted nor set (small, same family).

**The shape of the fix — split by mutability (lean).**
1. **Declarative config** (hooks, skills, permissions, settings, gitconfig
   identity) is *playbook-declared and materialized* like dotfiles, from the
   config source — versioned, reviewable, re-converged on `build` (ADR-0002/
   0006: declared, not hand-made). Note the engine already plans to bake
   guardrail hooks into built envs (protect-paths header, "Work 6") — the
   harness hooks ride the same mechanism.
2. **Mutable state** (memory, session history, creds) lives in the **T14 agent-
   home volume** — survives rebuild, never bind-shared with the host or another
   container (the session-journal corruption risk from the verification notes).

The boundary cuts cleanly at `~/.claude`: config materializes INTO the volume on
each build (docker cp through the mount), state accretes in it. Alternatives
considered in the verification notes: bind-mounting the host `~/.claude`
(rejected — shadows materialized config, shares mutable state across writers,
dead on macOS for creds); copying once at create (rejected — config drifts from
the source, exactly T7's class of bug).

**Open questions.**
- Playbook schema: does `dotfiles:` generalize (it already targets `$HOME`
  paths), or does a `harness:` section earn its keep (hooks/skills/permissions
  have semantics dotfiles don't — e.g. executable bits, per-project memory dirs)?
- Permissions/guard policy is *policy*: does its source belong in `rules:`
  (explicit-by-reference) rather than dotfiles? Interacts with the two-tier
  config (ADR-0004) — base-wide guards vs per-project allowlists.
- Memory seeding: start empty, or import the host's project memory once at
  volume creation (continuity vs a clean cut)?
- T10: everything here must target the parameterized `$HOME`, not `/root`.

**Session continuity is part of this thread (added 2026-06-10, at cutover).**
Session restarts are a certainty (model switch, rebuild, crash), so "how does a
fresh agent session regain project context" needs a reusable convention, not a
hand-written prompt each time. Three layers, by where each belongs:
1. *Now (repo convention):* a tracked-tree handoff doc the agent reads at
   session start (`.scratch/loom-dev-session-start.md` is the first instance) —
   works because the mounted repo is the one surface every session sees. The
   broader principle already holds: OPEN-THREADS + docs are the durable memory;
   chat is not.
2. *Playbook (this thread's scope):* the SessionStart continuity hook — the
   mechanism that produced devenv's session snapshots — is exactly the kind of
   harness config item (1) above materializes from the config source. ADR-0015
   should treat "agent regains context on session start" as a first-class
   harness-home requirement, alongside guard hooks: snapshot-on-end +
   orient-on-start, declared in the playbook, not hand-carried.
3. *Spec (later, human-authored):* the durable contract is PLAN's open item
   "agent-initiated container lifecycle with task continuity" — a RULES/SPEC
   clause + FR once the mechanism exists; not before (ADR-0013 spec→FR order).

Promote to: an ADR (harness-home strategy — config materialized vs state in
volume, INCLUDING session continuity hooks), then engine work (materialize
hooks/skills/permissions + executable bits), then FRs per behavior. Blocks T12
criterion 4; design together with the ADR-0014 addendum (T14) it builds on.
