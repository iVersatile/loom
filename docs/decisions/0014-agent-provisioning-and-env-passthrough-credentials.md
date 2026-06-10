# ADR-0014 — Agent provisioning + in-container credential login
**Date:** 2026-06-09   **Status:** Accepted (2026-06-09 — human acceptance via PR #7 merge instruction; drafted by agent for review per RULES §5/C3)

## Context
The playbook declares `agents:` (e.g. `claude-code`) and `env:` (e.g.
`ANTHROPIC_API_KEY`), but `build` acted on neither: `toolInstalls()` fed only
`resolution.Tools` to the container, so agents were *detected* and never installed
(open thread **T8**), and no credentials reached the container. The result was a
configured-but-uninhabitable container — `.claude/` materialised, but no `claude`
binary and no auth — which made the loom container unusable as a dev env (T12),
pushing work into the separate `devenv` sandbox.

The Claude Code CLI offers a **native installer**
(`curl -fsSL https://claude.ai/install.sh | bash`, no Node) landing the binary at
`~/.local/bin/claude`.

Three credential mechanisms were explored *and tested* this session; two failed:
- **Host creds-file mount/copy** — DEAD on a **macOS** host. Claude Code stores
  credentials in the **Keychain**, so there is no `~/.claude/.credentials.json` on
  the Mac to mount or `docker cp` (`ls` → No such file; `docker inspect … .Mounts`
  → `[]`). Works only on a Linux host that has a real creds file.
- **`CLAUDE_CODE_OAUTH_TOKEN` env passthrough** — authenticates **headless**
  `claude -p` only, **not** the interactive TUI (verified: token present in the
  container, TUI still prompted). Worse, `docker run -e VAR` stores the value in the
  container's `Config.Env`, leaking it into `docker inspect`, plus the operator's
  shell history and terminal scrollback.
- **Interactive in-container OAuth login** — WORKS. Completing the login inside the
  container writes `~/.claude/.credentials.json` to the container's **writable**
  home; `claude` then starts authenticated. Verified: an interactive session runs
  authenticated inside `loom-loom-dev`.

## Decision
**Agents (binary).** `build` installs declared agents during provision, mirroring
tool installs. claude-code uses the native installer; `~/.local/bin` is added to
PATH for login (`.profile`) and interactive (`.bashrc`) shells. The provision
sentinel digest folds in agents (`provisionDigest(tools, agents)`), so adding/
removing an agent re-provisions an existing container.

**Credentials — interactive in-container login (primary).** Authenticate by running
`claude` once inside the container and completing the OAuth flow. It persists to the
container's writable `~/.claude/.credentials.json` and refreshes in place. No secret
in loom's code, lockfile, image, logs, env, or Docker metadata. This is the only
path that authenticates the interactive TUI and the documented default.

**Credentials — secondary/optional, documented not default:**
- *Env token* (`CLAUDE_CODE_OAUTH_TOKEN` / `ANTHROPIC_API_KEY`) via `docker run -e
  NAME`: a **headless/CI-only** path (`claude -p`). Carries a leak cost — the value
  lands in `Config.Env`/`docker inspect` and the operator's shell history — so it is
  **not** in `loom.yml`'s default `env:` and must be added deliberately for CI.
- *Host creds-file mount* (`credsMount`, RO single file): a no-op when the host has
  no creds file (so it never breaks macOS); usable only on a Linux host that has a
  real `~/.claude/.credentials.json`.

## Alternatives considered
- **Generate `CLAUDE_CODE_OAUTH_TOKEN` via `claude setup-token`.** Tried; rejected as
  default: needs `claude` logged in on a controllable machine, mints a year-long
  token, and doesn't drive the interactive TUI — plus the leak cost above.
- **Bind-mount the whole `~/.claude`.** Rejected: shadows materialised
  `settings.json`/`statusline.sh`; and on macOS there's no source file anyway.
- **npm install (`@anthropic-ai/claude-code`).** Rejected for the default: drags
  Node into every container; the native installer avoids it.

## Consequences
- Positive: the loom container is inhabitable — authenticated interactive `claude`
  confirmed. No secret value in any loom artifact or Docker metadata on the primary
  path; creds live only at rest in the container home (same trust level as `devenv`).
- **Trade-off — creds lost on rebuild (T14).** The in-container login persists for
  the container's *life*; `build --force`/`teardown` (`docker rm`) wipes it →
  re-login required. A plain converge build keeps it. Durable persistence (a creds
  volume at `~/.claude`) is deferred to **T14**, alongside the harness-home work
  (memory/hooks/skills must also survive rebuild).
- Deferred: per-tool/per-agent **digest** pinning in the lock (**T5**); agent
  `resolved` is still host-probed (T5).
- The container user is still root with `$HOME=/root` (**T10**); creds/PATH paths are
  hardcoded to `/root` and must be parameterised when T10 lands.
- Revisit if: a secret-store integration (`apiKeyHelper`) is wanted, or a Linux host
  with a real creds file makes the file-mount a viable zero-touch path there.

## Addendum (2026-06-10) — durable agent home volume (T14)
The "creds lost on rebuild" trade-off is closed by a **named volume at
`~/.claude`** (`<container>-claude`), mounted at create when an agent is
declared. The in-container OAuth login writes `.credentials.json` into the
volume, so it survives `build --force`/`teardown` (`docker rm` keeps named
volumes) — re-login is needed only when the token actually expires, not on
every rebuild. The host creds-file mount (single file, RO) nests inside the
volume and still applies on Linux hosts. The volume is **not** removed by the
`volumes`/`reset` teardown tiers: wiping agent auth is the opt-in
`--clean-state` tier (SPEC-verbs teardown) — deleting credentials must be an
explicit choice, never a side effect. T15 (non-interactive, AI-first auth)
remains open; this addendum only makes the human login durable.
