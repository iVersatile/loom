# T8 — `agents:` are declared but never installed   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

Origin: the container loom builds (`loom-loom-dev`) has the materialized `.claude/`
config but **no `claude` binary** — investigating why the statusline/agent config
was inert revealed agents are never provisioned.

**Root cause (confirmed).** `ContainerSpec.Tools` is fed by `toolInstalls()`, which
copies only `resolution.Tools` — **agents are excluded** (`internal/engine/build.go:178-184`).
Agents are *only detected* (presence-probed, `internal/engine/detect.go:68-70`),
never installed. So `agents: [claude-code]` (base playbook) produces nothing; the
lock records `claude-code.resolved: ""`. The container has agent config without the
agent program.

**Why it matters.** This is the capability gap that makes `loom-loom-dev` an
uninhabitable dev env: the whole AI-first premise (ADR-0005) is that the container
hosts the agent, but loom installs none. Blocks "actually use the loom container."

**Scope also needs credentials.** A `claude` binary still needs auth to run. The
`devenv` sandbox gets this by bind-mounting the Mac's `~/.claude` (creds + settings).
loom must either bind-mount `~/.claude` creds or inject a token via the secret store
— **no baked secrets** (RULES). Installing the binary without solving creds is half
a fix.

**Options.**
1. Provision agents like tools: add an agent install path (per-agent source — npm/
   curl installer for claude-code, etc.), pin in the lock (`resolved`/`digest`),
   gate reinstall on an agent-set digest (cf. toolset digest).
2. Bind-mount the host agent install + `~/.claude` into the container instead of
   installing (lighter; mirrors how `devenv` works today).
3. Hybrid: install the binary (1), mount only credentials (2).

Lean: (3) — own the binary so the container is self-contained, mount only secrets.
PLAN link: realizes `docs/PLAN.md` → *Open items → "Working env for building Loom …
Claude Code in-container (dogfood)"* (its fallback "mount loom + claude into
/usr/local/bin in-container" is the agent half of that item).
Promote to: an engine capability + a creds decision (possibly an ADR — interacts
with ADR-0005 and the secret-store design); FR once `verify` covers agent install.

**Resolution (2026-06-09/10, PR #7 merged):** option (3) as leaned — `build`
installs declared agents (claude-code native installer, `~/.local/bin` on PATH);
the provision sentinel digest folds in the agent set. Credentials decided in
**ADR-0014 (Accepted)**: interactive in-container OAuth login primary; env-token
demoted to CI-only; host creds-file mount a Linux-only no-op-on-mac secondary.
Covered by **FR-BUILD-006**. Durability of the login → T14 (resolved); its
human-only nature → T15 (open).
