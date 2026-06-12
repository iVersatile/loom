# ADR-0018 — Harness trust/opt-in flags are declared playbook config, materialized at build
**Date:** 2026-06-12   **Status:** Proposed (authorship chain: **human-decided** — 036 ruled "2-plus" in-session 2026-06-12, queue row merged PR #103 — → **agent-transcribed** — this ADR + the `trust:` implementation — → **human-accepted**; acceptance = PR merge, per RULES §5/C3)

## Context
The agent harness stores its trust/opt-in flags (auto-mode acceptance,
`projects['<path>'].hasTrustDialogAccepted`, onboarding state) in
`$HOME/.claude.json` — a **sibling** of the mounted `~/.claude` agent-home
volume. The sibling lives on the container overlay, so it dies on every
container recreate. Field evidence (envelope 036, both seats,
advisor-verified via /proc/mounts): each recreate cost a human a manual
trust re-flip, logged in flips.log (T23) — twice within two days for the
Writer seat alone. The repo's `.claude/settings.json` `defaultMode: auto`
is inert without that acceptance, so the auto-trial silently degrades on
every recreate. ADR-0017's target state (Writer push-to-branch) names 036
as its blocker.

SPEC-playbook's harness clause (ADR-0015 decision 2) said mutable state is
never a valid `harness:` target — written to stop the engine from managing
session history and credentials, but read literally it also forbade the fix.

## Decision
1. **Trust/opt-in flags are declared config, not mutable state.** The
   playbook owns them the way it owns harness-home config (ADR-0014/0015):
   a `harness.<agent>.trust:` dotfiles reference, materialized WHOLE-FILE to
   `$HOME/.<agent>.json` through the existing staging pipeline (write-if-
   changed; T7 home-digest sentinel; build/materialize audit entries;
   staged-home drift grades it for plan/doctor).
2. **Declare-or-rederive, not mount-widening.** The fix is *not* a wider
   volume or a hand docker edit: declared state is re-derived at build, and
   everything else the live `$HOME/.<agent>.json` accumulates is rederivable
   cache that may be overwritten by a home re-sync or die with the container
   (devcontainer/Codespaces precedent; ruled "caches may die").
3. **Credentials stay per-session injection** (ADR-0014) — nothing about
   this decision moves secrets into declared config.
4. **Trust changes ship as playbook edits.** A trust flip is a PR to the
   declared file; flips.log (T23) keeps recording flips that happen outside
   that flow, and divergence between the two is drift evidence.

## Alternatives considered
- **Mount-widening** (volume or bind over `$HOME/.<agent>.json`): rejected
  by ruling — persists unaudited mutable state wholesale, inverts
  declare-or-rederive, and survives nothing the playbook can reason about.
- **Hand docker edits post-recreate** (status quo): rejected — that is the
  human re-flip loop this fixes; unauditable, forgotten under load.
- **Key-merge into the live file** (engine reads, deep-merges declared keys,
  writes back): deferred — preserves runtime cache but adds an engine
  read-modify-write path into live container state, contradicting the
  Phase-1 whole-file/no-key-merge doctrine (SPEC-playbook Open question 1).
  Revisit alongside settings.json key-merge if cache loss proves costly.

## Consequences
- Container recreate → home re-sync re-materializes the declared flags →
  the seat comes up opted-in with zero manual re-flips (envelope 039
  acceptance; flips.log should gain no new manual entries for this cause).
- A home re-sync overwrites the live `$HOME/.<agent>.json`: runtime-
  accumulated per-project state (history, counters, ad-hoc allows) is
  declared cache and is lost by design. A mid-session re-sync can also be
  overwritten *back* by the harness's own state writes until the next
  session start — durable convergence is at session boundary, not instant.
- The declared key set is config: if a harness version consults additional
  opt-in keys, extending `config/dotfiles/claude/trust.json` is a config
  edit, no engine change.
- Unblocks ADR-0017 decision 1 (Writer push-to-branch at 036-unblock) once
  a recreate validates the acceptance criterion.
- Revisit if: the harness moves trust flags out of `$HOME/.<agent>.json`;
  key-merge lands for settings.json (fold trust into it); or cache-loss
  from whole-file overwrite shows up as real friction in the trial.
