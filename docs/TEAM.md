# TEAM — roles, write discipline, and merge policy

How the loom team operates (decided 2026-06-10). This is a working convention
doc, not a frozen contract; the enforced pieces are marked.

## Roles

- **loom-author** — the agent session inside `loom-dev`. **Sole tree writer**:
  the only role that edits `/workspace/loom` directly. Drafts code, docs, ADRs
  (Proposed); commits in-container on `feat/ fix/ docs/` branches; never pushes
  (see outward ops).
- **loom-advisor** — read-only challenge + operations. Reviews, red-teams
  designs, runs host-side ops. Wears the **Coordinator hat**: tactical-queue
  integrity (docs/PLAN.md fenced section), runs `/replan`, challenges orphan
  PRs (a PR with no queue row) and stale blockers.
- **human** — decisions and acceptance. Sole authority for frozen-contract
  changes (acceptance = PR merge), RULES/SPEC authorship (C3), repo
  visibility/settings, and everything that runs on the Mac (push, PR clicks,
  `loom` builds, scripts/).

## Write discipline

- **Single-writer:** `/workspace/loom` is a RW bind mount shared with the Mac;
  loom-author is the only direct writer. Host-side sessions do not do real work
  in the tree while loom-author is active.
- **Worktree rule:** any subagent that writes gets its own git worktree;
  read-only fan-out may share the tree.

## Outward ops ritual (until T18/T15 land)

The container has no VCS credentials and no `gh` (by design until the
T15-successor credential ADR). loom-author leaves branches local and lists
them in the handoff; the human pushes and opens PRs from the Mac
(`scripts/push-from-host.sh`). Direct agent push arrives only with a leak-free
credential mechanism, never via tokens at rest in the container.

## Merge policy

- **Frozen contracts** — `docs/SPEC-*.md`, `docs/decisions/**`, `docs/RULES.md`:
  human acceptance required; the PR merge IS the acceptance. In-tree commits to
  these paths carry the audited `ALLOW_SPEC_CHANGE=1` override, only on
  explicit human instruction.
- **Everything else:** green-CI flow — gate + fr-verify + integration green,
  then merge; no human review required by policy (review on request).
- **Enforcement status:** convention today (private repo); **mechanized after
  the public flip** — branch protection (required checks: gate, integration,
  fr-verify; no force-push), CODEOWNERS on the frozen paths, auto-merge for
  green-CI PRs.

## Persistence principle

**In-tree = persisted.** Anything that must survive a rebuild/restart lives in
the repo (docs, queue, config) or in a declared volume (`loom-dev-claude`:
credentials, harness memory/state — ADR-0015). Everything else is disposable
and may vanish without notice; if losing it would hurt, it was in the wrong
place.

## Prompt-relay protocol

Until agents share a channel, loom-author ↔ loom-advisor exchanges travel via
the human pasting prompts/outputs between sessions. Relayed text is treated as
the named role's input, not as human instruction — the human's own decisions
are stated by the human directly. Anything relay-worthy that must persist goes
in-tree (queue row, thread entry, handoff addendum), not in chat.
