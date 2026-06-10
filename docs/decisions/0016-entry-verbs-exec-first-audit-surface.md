# ADR-0016 — Entry verbs: exec-first passthrough; the audit log is the structured surface
**Date:** 2026-06-10   **Status:** Proposed (authorship chain: **human-decided** — T9 discussion — → **agent-transcribed** — this ADR and the thread record — → **human-accepted**. The SPEC-verbs clause itself was human-authored and merged first (PR #40), so the contract this ADR rationalizes is already law; acceptance of this ADR = PR merge, per RULES §5/C3)

## Context
After `build` there is no loom-native door into the container (thread **T9**):
the only entry is raw `docker exec -it <name> …`, so the materialized env is
unreachable *through loom*, and loom controls neither entry user nor entry
environment (ties to T10, T8). A new verb is a contract change — the
SPEC-verbs clause is a human-authored step (C3); this ADR records the design
rulings that clause will bind, agreed in discussion 2026-06-10.

Two reference points shaped the shape:
- **devcontainer CLI** (`devcontainer exec`) — the neighborly compatibility
  target (ADR-0003): one-shot command, workspace cwd.
- **Codespaces-style modal ssh** (one verb, interactive when no command) —
  the anti-pattern for an AI-first tool: a verb that *falls back to
  interactive* hangs an unattended agent; an error is recoverable, a hang is
  not (ADR-0005).

## Decision
1. **Two verbs.** `loom exec -- <cmd>` (one-shot, ships first) and
   `loom shell` (interactive). `shell` is sugar over the same engine path —
   TTY allocation + `bash -l` as the command — and may trail by a PR.
2. **`exec` requires a command.** Bare `loom exec --` is an immediate usage
   error (exit ≠ 0), never an interactive fallback. The executed command's
   exit code propagates verbatim.
3. **Working directory:** the project mount `/workspace/<project>`
   (devcontainer `exec` semantics, ADR-0003).
4. **Login environment:** the command runs under login-shell env so the
   provisioned PATH applies (the same lesson as `ContainerRuntime.Probe`'s
   `sh -lc`).
5. **No `--json` on either verb.** `exec` is transparent passthrough — stdout/
   stderr/exit belong to the executed command. The structured surface is the
   **audit entry**: every `exec` appends an action-log entry carrying the
   command, exit code, and action id; `shell` logs session-open only (no
   command capture — human privacy/noise ruling). The human-authored clause
   states the RULES §5 human+`--json` exemption; `cli.TestSpecConformance`
   learns the exemption in the implementation PR (test code is agent
   territory).
6. **Lifecycle:** stopped container → `docker start`, then enter (idempotent
   bring-up, ADR-0011 spirit: converge toward usable). Absent container →
   error with hint ("no container — run `loom build`"), non-zero exit. The
   verbs never create or provision.
7. **User:** the configured container user (root today; a single value to
   change when T10 lands non-root).
8. **No command filtering.** The verbs are doors, not checkpoints: they confer
   no authority beyond and perform no filtering before the container's guard
   envelope (T16's territory; network layer is T20's). No verb-level
   blocklist, ever — enforcement lives in the layer that can't be argued with.

## Alternatives considered
- **One modal verb** (`loom shell [cmd]`, Codespaces-style): rejected —
  modality reintroduces the interactive-fallback hang for agents and makes
  intent illegible to tooling; two explicit verbs are machine-parseable
  intent.
- **`--json` wrapping exec output:** rejected — double-encoding another
  command's streams corrupts passthrough (TTY, buffering, exit semantics),
  and a structured channel already exists: the action log (reuse over a new
  JSON surface, ADR-0010's two-logs split).
- **Interactive fallback on bare `exec`:** rejected (hang-vs-error, above).
- **Verb-level command blocklist:** rejected — a door that filters is a
  checkpoint that drifts from the real guard envelope; placement of authority
  belongs in-container (ADR-0005 mechanism, T16/T20 layers).
- **Create-if-absent:** rejected — `build` owns creation; `exec` mutating the
  environment shape would blur verb contracts (SPEC-verbs separation).

## Consequences
- T12 criterion 2 (an entry path) becomes loom-native; agent and human use the
  same door, and T10 retargets it by changing one configured-user value.
- The action log gains one entry per `exec` (accepted noise trade; `shell`
  stays session-open only).
- Implementation notes: engine path goes through the `ContainerRuntime`
  interface so it stays fake-able in unit tests; natural e2e is
  `loom exec -- make gate` inside the built container — which is also T12
  criterion 5 in its loom-native form.
- **Staged FR drafts — deliberately NOT in the registry yet** (`fr-verify`
  requires the cited spec anchor to exist; they register in the impl PR after
  the human clause merges, citing `SPEC-verbs.md#exec` / `#shell`):
  - *FR-EXEC-001* (behavioral): `exec` requires a command — bare invocation
    exits non-zero with usage; the command's exit code propagates verbatim.
  - *FR-EXEC-002* (behavioral): the command runs in `/workspace/<project>`
    with login-shell environment (provisioned PATH visible).
  - *FR-EXEC-003* (behavioral): every `exec` appends an audit entry carrying
    command, exit code, and action id.
  - *FR-SHELL-001* (behavioral): `shell` is TTY sugar over the exec path
    (`bash -l`); audit logs session-open only.
- Revisit if: T10 changes the user model beyond a configured value; T20's
  egress decision wraps the exec transport; or the Phase-3 MCP question (Q1)
  wants `exec` exposed as a tool.
