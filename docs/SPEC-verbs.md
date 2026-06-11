# SPEC — Verb Contracts

> Status: **frozen for Phase 1** (reviewed 2026-06-08). The engine is mechanism
> (ADR-0006); these verbs reconcile reality to the playbook. Every verb supports
> human output and `--json` (ADR-0005). All verbs are idempotent and safe to
> re-run.

## Global conventions

- `loom <verb> [target] [flags]`
- `--json` on every verb → structured output to stdout, logs to stderr.
  Exemption: `exec` and `shell` — transparent passthrough / interactive; their
  structured surface is the action log (see their sections).
- No `--dry-run`: `plan` is the one preview path (read-only, exit 2 on drift).
  A flag alias was removed (T6) after it shipped mutating — one preview surface
  keeps the read-only promise enforceable.
- Exit codes: `0` success / no-op, `1` error, `2` "changes needed" (for `plan` in
  check-mode, so CI/agents can gate on drift).
- Never perform a prohibited/irreversible action without an explicit gate
  (mirrors the environment's own guardrail philosophy).

## detect

Read current reality; never mutates. Foundation for continuity (scenario 2) and
for an agent orienting itself.

- Scans: installed tools, agent CLIs, present keys/credentials across known
  locations (Keychain, shell exports, config files, env), existing projects,
  drift vs the playbook.
- Human: a grouped report. `--json`: a state document.
- **Continuity:** `--emit-playbook` writes a *draft* base playbook representing the
  detected machine (the bridge to no-information-loss — review then commit).
- Credential handling: **detect + report by default** (lists what/where, never
  moves secrets); `--migrate` consolidates into `.env` only with confirmation +
  a diff first.

```json
// loom detect --json (shape)
{ "tools": [{"name":"uv","present":true,"version":"0.4.1"}],
  "agents": [{"name":"claude-code","present":false}],
  "credentials": [{"name":"ANTHROPIC_API_KEY","found_in":["keychain","~/.zshrc"]}],
  "projects": [{"name":"prompiler","path":"...","stack":"python"}],
  "drift": [{"tool":"ruff","want":"0.5.2","have":null}] }
```

## plan

Compute the diff between current state and the playbook. Never mutates. The
`terraform plan` analog — the trust surface for autonomous runs.

- Human: a readable change list. `--json`: a structured plan.
- Exit `2` if changes are needed (lets CI/agents gate), `0` if already converged.
- Must be runnable by an agent *before* `build`/`update` to preview blast radius.

```json
{ "create": [{"kind":"container","name":"prompiler-dev"}],
  "install": [{"tool":"ruff","from":null,"to":"0.5.2"}],
  "remove": [{"tool":"flake8","reason":"absent from playbook"}],
  "noop": ["python@3.11"] }
```

## build

Materialize the playbook into reality: resolve intent → write lockfile → produce
container(s) (one per project, shared base image) + the two-tier config.

- Idempotent: re-running converges; already-correct items are no-ops.
- **Presence is not convergence (ADR-0011):** an existing container that is
  under-provisioned (a prior build interrupted mid-provision) or whose tool set
  drifted is *re-provisioned* back to the playbook — container status `converged`,
  audit action `container.reconcile` — not left wedged for `--force` to clear.
- Writes/updates `loom.lock`.
- Flags: `--force` (rebuild from scratch), `--stack`/`--overlay` (first scaffold
  only; thereafter read from the project playbook).

```json
// loom build --json (shape)
{ "resolved": { "tools": { "ruff": {"resolved":"0.5.2","source":"uv"} } },
  "lock_written": true,
  "container": {"name":"loom-dev","image":"...@sha256:...","status":"created"},  // status: created | exists | converged

  "materialized": ["~/.claude/settings.json","~/.claude/statusline.sh","~/.bashrc.d/10-prompt.sh"],
  "actions": ["<audit-entry-id>", "..."],
  "result": "created" }   // created | converged | noop
```

## update

Reconcile a running environment to the *current* (edited) playbook — apply only
the delta. This is what makes long-lived evolution work (tools added/removed,
stack changed, workflow enforced): edit playbook → `update` → converge.

- Internally: `plan` then apply the delta. To preview, run `plan` (no `--dry-run`).
- Removal is real: a tool dropped from the playbook is uninstalled.
- Re-resolves and rewrites `loom.lock`.

## teardown

Tiered, mirrors the validated bundle teardown. Container-only levels never touch
Mac-side code/config.

- Levels: `stop` | `volumes` | `reset` (container / +volumes / +image).
- Opt-in Mac-side: `--clean-state` (agent auth, memory, logs),
  `--wipe-project` (whole folder; typed confirmation, not bypassable by `--yes`).
- `--json` reports what was removed.

```json
// loom teardown --json (shape)
{ "level": "volumes",
  "removed": { "containers": ["loom-dev"], "volumes": ["loom-dev-data"], "images": [] } }
```

## exec

Run one command inside the project container — the agent-facing door
(ADR-0005). Scripts, CI, and agents drive the container through this verb; the
dogfood loop's loom-native form is `loom exec -- make gate`.

- `loom exec -- <cmd> [args…]`. The command is **required**: a bare
  `loom exec` is a usage error (non-zero exit), never an interactive
  fallback — an unattended caller must fail loudly, not hang on a TTY.
- Exit code: the command's own, propagated verbatim.
- Working directory: the project mount (`/workspace/<project>`) — matches
  `devcontainer exec` semantics (ADR-0003 compatibility).
- Environment: login-shell env, so the provisioned PATH applies. Runs as the
  configured container user.
- Output: transparent passthrough — stdout/stderr belong to the executed
  command; no `--json` (exempt from the global convention). The structured
  record is the **audit entry**: every `exec` appends an action-log entry
  carrying the command, its exit code, and the entry id.
- Lifecycle: a stopped container is started, then entered (idempotent
  bring-up); an absent container is an error with the hint to run
  `loom build`, non-zero exit. `exec` never creates or provisions.
- Doors, not checkpoints: `exec` confers no authority beyond, and performs no
  filtering before, the container's own guard envelope (the hooks and
  permissions `build` materializes). Guarding lives inside the container; the
  verb is plain.

## shell  (staged — ships after exec)

Interactive entry for the human topologies: sugar over `exec`'s engine path —
a TTY plus a login shell.

- `loom shell` → interactive login shell in the project container, as the
  configured user, in the project mount.
- Same lifecycle and no-filtering posture as `exec`; exits with the shell's
  exit code.
- No output contract (interactive; exempt from `--json`). Audit: one
  session-open entry — commands typed in the shell are not captured.

## entry: bootstrap  (execution-entry contract; added 2026-06-11, human-decided — C3)

`bootstrap/loom-bootstrap.sh` is the **pre-trust surface**: the only code
that runs before any guardrail exists. Its whole contract is
detect-situation → ensure-engine-present → exec-engine; it stays small and
auditable by construction (ADR-0008).

- **Scope (strict):** no engine logic, no provisioning, no environment
  mutation beyond building `bin/loom`.
- **Engine presence, in order:** (i) an executable at `LOOM_BIN` (default
  `bin/loom`) is used as-is; (ii) else, with a Go toolchain >= 1.26, build
  from `./cmd/loom`; (iii) else fail per the frozen failure contract.
  Bootstrap NEVER fetches binaries or any artifact over the network.
- **Frozen failure contract:** no engine + no Go ⇒ exactly one line to
  stderr naming BOTH remedies — install Go >= 1.26, or provide a prebuilt
  binary at `bin/loom` or via `LOOM_BIN` — then exit 1.
- **Audit exemption (explicit):** bootstrap predates the engine audit log
  and is exempt by declaration; the first engine action after handoff is
  audited as usual.
- **Documented future amendments** (named so they arrive as decisions, not
  drift): (a) native Windows entry (pwsh) — its own clause amendment when
  the windows-dev topology activates; (b) prebuilt-binary fetch — an
  amendment explicitly **gated on T20** (egress thread).

## import  (ADR-0003, staged)

Ingest an existing `devcontainer.json` → a Loom project-tier playbook (Stage 1
compatible), ready to be enriched with base/overlay/rules (Stages 2–3).

- Deterministic for the mechanical fields (image, features, ports, env, commands).
- Intent inference for awkward fields may use an AI skill (flagged, optional).

## export  (later, lossy)

Emit a `devcontainer.json` from the environment fields of the playbook for
handoff to VS Code/Codespaces. Policy/intent (rules, hooks, AI-context) do not
map and remain in repo docs. Deterministic; AI only for residual judgment cases.

## doctor / verify

Self-check: are required tools present (jq, etc.), are hooks executable, is the
lockfile consistent with the playbook, are guardrails active. `--json` for agents.

## Cross-cutting requirements (apply to all verbs)

- **Idempotent & recoverable** — partial failure leaves a re-runnable state.
- **Auditable** — append an entry to the action log (what/when/diff) for every
  mutating verb; an autonomous run must be reviewable after the fact. Shape and
  location are frozen below (Action log).
- **Observable** — every mutating verb also writes a diagnostic log (raw step
  output) for troubleshooting; see Diagnostic log (ADR-0010). Distinct from the
  audit log: *how it ran*, not *what changed*.
- **Guarded** — mutating verbs respect the same deny/gate philosophy as the
  environment; an agent cannot weaken guardrails or perform prohibited actions
  via a verb without an explicit gate.

## Action log (frozen 2026-06-08)

`build` is the first mutating verb (Phase 1), so the log is decided now against a
real append site. **Per-project, append-only JSONL** at `<repo>/.loom/actions.log`.
Per-project (not machine-wide) so the trail travels with the repo and matches
container-per-project isolation (ADR-0001). One JSON object per line — append-only is
crash-safe (one line = one committed action) and trivially greppable by an agent.

```json
{ "ts":"2026-06-08T00:00:00Z", "verb":"build", "action":"container.create",
  "target":"loom-dev", "before":null, "after":{"image":"...@sha256:..."},
  "result":"created", "actor":"cli" }   // actor: cli | agent
```

## Diagnostic log (ADR-0010)

Separate from the audit log: a per-project, free-form **diagnostic log** at
`<repo>/.loom/logs/<verb>.log`, written by every mutating verb, capturing raw step
output (the docker commands, provisioning `set -x` trace, etc.) for
troubleshooting. **Contract:** it exists, its location, and that mutating verbs
produce it. **Not frozen:** its content/format — it is an operability aid and may
change (ADR-0002 thin surface). Gitignored runtime state, like the action log.

Every mutating verb appends one entry per discrete action. The `actions` array in a
verb's `--json` output carries the entry ids written during that run.

## Deferred (decided not to decide yet)

1. **MCP vs direct verbs — deferred to Phase 3, deliberately.** Phase 1 agents
   operate the verbs directly via `--json`; the verb surface is the contract. We
   revisit an MCP wrapper once the verbs are stable, so we wrap a known-good
   surface rather than designing two at once (PLAN Phase 3, ADR-0005).

## Open questions

1. Should `plan` check-mode (`exit 2`) be the default in CI contexts?
2. **Self-lifecycle continuity:** when the operating agent runs *inside* the
   container, how does `teardown`/restart preserve and rehydrate the agent's
   task-on-hand context? Candidate shape: task state lives on a durable surface
   outside the container (host-mounted state dir / git / the cloud durable store,
   ADR-0007), the agent checkpoints before a self-initiated down, and `build`
   (or `start`) exposes a rehydrate hook on bring-up. Drives toward
   agent-debug-and-fix (PLAN Phase 3). See PLAN open items.
