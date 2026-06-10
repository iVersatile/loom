# TOPOLOGY — named operating topologies

Reference topologies for where loom runs and who operates it. Other documents
cite these by name (`mac-dev-topology`, `windows-dev-topology`,
`ai-user-topology`) instead of re-describing the environment; append-only
records (OPEN-THREADS, accepted ADRs) keep their historical inline wording.
These map to the CHARTER's "one installer, many situations" — a topology is a
*situation* the engine must converge correctly in.

## mac-dev-topology — current, the Phase-1 dogfood

The validated topology; everything in Phase 1 was proven here.

- **Host:** a Mac running Docker Desktop (linuxkit VM). Holds what cannot or
  must not live in a container yet: docker control, the cross-compiled
  `bin/loom-darwin-arm64`, VCS credentials + `gh` (outward ops — the T18
  ritual, `scripts/push-from-host.sh`), and the visibility/settings clicks
  (docs/TEAM.md: human role).
- **Containers:** one per project, `<project>-dev` (ADR-0001/T11), built and
  converged by loom; the project repo bind-mounts RW at
  `/workspace/<project>` (T13) — shared with the host, hence the
  single-writer discipline (docs/TEAM.md).
- **Credentials:** Claude Code on the Mac stores tokens in the **Keychain**,
  so there is no host creds file to mount (ADR-0014: that path is dead on
  macOS). The working mechanism is one in-container OAuth login persisted in
  the agent-home volume `<container>-claude` (ADR-0014 addendum / ADR-0015:
  state accretes in the volume).
- **Operator mix:** the human drives host-side ops; the loom-author agent
  session lives *inside* `loom-dev` (docs/TEAM.md). `devenv`, the predecessor
  sandbox, is archived at the T12 cutover and deleted archive+14d.

## windows-dev-topology — sibling, declared but NOT validated

The expected shape on a Windows host, recorded so design choices keep it
reachable. Nothing here has run; treat every line as a hypothesis to verify
when a Windows machine dogfoods it (Phase 2+).

- **Host:** Windows with Docker Desktop (WSL2 backend). The engine binary is
  either a `loom-windows-*` build or the Linux binary run from WSL2 — the
  bootstrap is POSIX sh (ADR-0008), so WSL2 is the natural shell home, not
  PowerShell.
- **Expected deltas to verify:**
  - *Credentials:* Windows Credential Manager plays the Keychain's role — the
    host creds-file mount is presumably dead here too, and the in-container
    login + volume mechanism (ADR-0014/0015) should carry unchanged, being
    host-agnostic. Verify, don't assume.
  - *Bind mounts:* path mapping (`\\wsl$`, drvfs/9p) changes mount
    performance and **executable-bit semantics** — materialized hooks and
    `statusline.sh` rely on preserved +x (T16 engine work must not regress
    here).
  - *Line endings:* `core.autocrlf` must not rewrite materialized dotfiles or
    POSIX scripts; the home-digest sentinel (T7 fix) would read CRLF drift as
    perpetual non-convergence — a useful canary, but only if the cause is
    understood.
- **Status:** unsupported until a claims script equivalent passes there; the
  point of naming it now is that no new design decision may silently assume
  Keychain or macOS paths.

## ai-user-topology — north star, the agent as sole operator

The CHARTER's terminal state: a loom environment operated by an AI agent with
**no human in the loop** (ADR-0005: the agent is a first-class user). Today's
mac-dev-topology contains a partial embedding of it (loom-author inside
`loom-dev`); these are the deltas that remain:

- **Placement:** the agent runs inside the env it operates (today) or beside
  it (cloud sandbox sibling, ADR-0007). Lifecycle operations on the container
  the agent occupies must run from outside it — an agent never rebuilds or
  tears down its own floor (the loom-dev rule), so autonomous lifecycle needs
  task state persisted outside the container and **rehydrated** on bring-up
  (PLAN open item "agent-initiated container lifecycle"; ADR-0015
  session-continuity hooks are the in-container half).
- **Credentials by mechanism, not ritual:** secret store + per-use helper for
  both the model API and VCS (T15 lean, T15-successor ADR) — no human OAuth
  dance, no secret at rest in any loom artifact, nothing an agent could
  exfiltrate from loom's own surfaces (the ADR-0005 design test).
- **Outward ops:** push/PR/merge flow through mechanized policy (branch
  protection, CODEOWNERS, green-CI auto-merge — docs/TEAM.md) instead of the
  human-relayed T18 ritual; the human remains the acceptance authority for
  frozen contracts only.
- **Blockers, tracked in the PLAN queue:** T15 successor (credentials), T9
  (entry verb; human spec clause), T10 (non-root), the task-continuity spec
  (human-authored, C3).

## Citing these

Write `mac-dev-topology (docs/TOPOLOGY.md)` on first reference in a document,
bare names after. When a behavior is topology-specific, say which topology it
was verified in — "works" with no qualifier means mac-dev-topology until a
second topology has a passing claims script.
