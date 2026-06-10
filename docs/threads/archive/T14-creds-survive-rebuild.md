# T14 — agent credentials are lost on every rebuild   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

Origin: `loom-dev` was made usable (T8) by completing an **interactive OAuth login
inside the container** — the only path that works for the interactive TUI after the
file-mount and env-token routes failed (see below). Confirmed working: `claude`
starts authenticated in `loom-loom-dev`.

**The gap.** That login writes `/root/.claude/.credentials.json` in the container's
**writable** home, so it persists for the container's *life* — but `build --force` /
`teardown` (`docker rm`) destroys the container and the file with it. A plain
*converge* build does NOT (the container survives), so the loss trigger is
specifically a forced rebuild / teardown. Net: **re-login required after every
`--force`.** Acceptable as a stopgap; not acceptable long-term.

**Why the simpler paths don't solve it (verification findings, this session).**
- *Host creds file mount/copy* — DEAD on a **macOS** host: Claude Code stores creds
  in the **Keychain**, so there is no `~/.claude/.credentials.json` to mount or
  `docker cp` (`ls` on the Mac → No such file; `docker inspect … .Mounts` → `[]`).
  Would work only on a Linux host that has a real creds file.
- *`CLAUDE_CODE_OAUTH_TOKEN` env passthrough* — works for **headless** `claude -p`
  only, **not** the interactive TUI; and `docker run -e VAR` stores the value in the
  container's `Config.Env`, leaking it into `docker inspect` + shell history. Dropped
  from `loom.yml` default; kept as an optional CI-only path (ADR-0014).

**Options (no decision yet).**
1. **Persist a creds volume** — bind a small named volume at `~/.claude` (or just the
   creds file) so it survives `docker rm`; re-login only when the token actually
   expires. Simplest durable fix.
2. **Re-seed creds on build** — have loom copy a stored creds file back into the
   container home on (re)build, sourced from a host location or secret store.
   Depends on a host creds file existing (false on macOS) → weak unless paired with
   a loom-managed creds store.
3. **`apiKeyHelper`** — a script in `settings.json` that fetches a key per request
   from a secret manager; most secure, most setup.

Lean: (1) for the dogfood loop now (a creds volume excluded from the reset tier),
revisit (3) for a real secret-store integration. Interacts with the **harness-home**
thread (memory/hooks/skills also need to survive rebuild) and **T13** (mounts).

PLAN link: a specific case of `docs/PLAN.md` *Open items → "Agent-initiated container
lifecycle with task continuity"* — that item observed (2026-06-09) a `docker stop
devenv-dev` losing a live session mid-task; creds are one piece of the container
state that must **persist outside and rehydrate on bring-up** (memory/session
continuity are the rest, in the harness-home thread).
Promote to: an engine change (persist `~/.claude` across rebuild) + an ADR-0014
addendum; FR once covered.

**Resolution (2026-06-10, PR #10 merged):** option (1) — a named volume
`<container>-claude` mounts at `~/.claude` when an agent is declared, so the
in-container login survives `--force`/`teardown`; re-login only on real token
expiry. Deliberately excluded from the `volumes`/`reset` teardown tiers (agent-
auth wipe is the opt-in `--clean-state` tier). Recorded in the ADR-0014
addendum; covered by `engine.TestCreateRunArgs`. Option (3) `apiKeyHelper`
remains the T15 path; the mutable-state half of harness-home now rides this
volume (see T16).
