# SPEC — Verb Contracts (draft)

> Status: draft for discussion. The engine is mechanism (ADR-0006); these verbs
> reconcile reality to the playbook. Every verb supports human output and
> `--json` (ADR-0005). All verbs are idempotent and safe to re-run.

## Global conventions

- `loom <verb> [target] [flags]`
- `--json` on every verb → structured output to stdout, logs to stderr.
- `--dry-run` where an action would change state (alias of `plan` semantics).
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
- Writes/updates `loom.lock`.
- Flags: `--force` (rebuild from scratch), `--stack`/`--overlay` (first scaffold
  only; thereafter read from the project playbook).

## update

Reconcile a running environment to the *current* (edited) playbook — apply only
the delta. This is what makes long-lived evolution work (tools added/removed,
stack changed, workflow enforced): edit playbook → `update` → converge.

- Internally: `plan` then apply the delta. `--dry-run` == `plan`.
- Removal is real: a tool dropped from the playbook is uninstalled.
- Re-resolves and rewrites `loom.lock`.

## teardown

Tiered, mirrors the validated bundle teardown. Container-only levels never touch
Mac-side code/config.

- Levels: `stop` | `volumes` | `reset` (container / +volumes / +image).
- Opt-in Mac-side: `--clean-state` (agent auth, memory, logs),
  `--wipe-project` (whole folder; typed confirmation, not bypassable by `--yes`).
- `--json` reports what was removed.

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
- **Auditable** — append an entry to an action log (what/when/diff) for every
  mutating verb; an autonomous run must be reviewable after the fact.
- **Guarded** — mutating verbs respect the same deny/gate philosophy as the
  environment; an agent cannot weaken guardrails or perform prohibited actions
  via a verb without an explicit gate.

## Open questions

1. Does the AI operate verbs directly, or through an MCP server wrapping them?
   (MCP would give a typed tool surface — possibly the cleanest agent interface.)
2. Action-log location/format (per-project vs machine-wide; JSONL?).
3. Should `plan` check-mode (`exit 2`) be the default in CI contexts?
