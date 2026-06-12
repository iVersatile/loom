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
- **Shared-`.git` rule (LL-010):** host-side git operations touch the same
  `.git` the container uses — ref moves (pull/merge) count as writes;
  coordinate before batch ref-rewrites. Never invoke git with
  `GIT_DIR`/`GIT_WORK_TREE` exported toward the shared repo (a host-worktree
  commit attempt with those set leaked through the pre-commit gate into test
  fixtures and clobbered the real `.git/config`). The gate and fixtures are
  now hermetic to `GIT_*` (LL-010 layers), but the env hygiene rule stands —
  mechanism backstops the rule, it doesn't replace it.
- **No blind ops — check HEAD first:** every host-side git command sequence
  starts with `git rev-parse --abbrev-ref HEAD`, always. "I assumed main" is
  how both 2026-06-10 shared-tree incidents started (the GIT_*-exported
  worktree commit, and a rebase against a moved ref); thirty characters of
  verification beats an evening of recovery.
- **Commit identity:** every topology (docs/TOPOLOGY.md) commits as the GitHub
  noreply address (`1323991+iVersatile@users.noreply.github.com`) — Option C,
  decided 2026-06-10: historical gmail commits stay (rewrite rejected: breaks
  SHA refs in docs, destroys the PR acceptance trail, and `refs/pull/*` keeps
  old commits reachable regardless); account-side noreply + push-block are
  set. The `user.email` that T16's harness-home materialization declares
  (`~/.gitconfig`, ADR-0015 item 1) must be this noreply address, never a
  personal one.

## Agent operating mode: safe-auto

loom-author runs in **safe-auto** = `permissions.defaultMode: "acceptEdits"`
plus this repo's allow/deny lists (`.claude/settings.json`). Human-accepted
after the auto-mode evaluation (2026-06-10; advisor-verified: deny rules block
in ALL modes including bypass; `defaultMode` is a real harness key; custom
named modes don't exist). **Full bypass (`bypassPermissions`) stays banned.**
Known limit, tracked as T20: allowed `go test`/`go build` execute arbitrary
code incl. network I/O, so the deny floor cannot stop exfiltration at the
network layer — container-level egress restriction is the mechanism answer.
Full-auto clearance is event-driven (queue row: re-run evaluation when T16
hooks + T10 non-root + T20 land), not scheduled.

**Never-auto floor** — these prompt by design, permanently, in every mode and
under every future allowlist or hook expansion. Rationale: LL-010 (one leaked
`git config` write bricked every git client on every machine) and T20 (the
network layer is where the deny floor's guarantees end):
- credential paths (`~/.claude/.credentials.json`, `.env*`, any secret at rest)
- network egress (`curl`/`wget`/`nc`/`ssh`/`scp`, WebFetch/WebSearch)
- `git config` writes — any scope
- ref-surgery and history rewrites (`update-ref`, `reset --hard`,
  `filter-branch`/`filter-repo`, `rebase`, `push --force*`)
- container self-destruction (`loom teardown`, `loom build --force` from
  inside the container)
Prompt-volume work (allowlists, the compound allow-hook) may grow what's
*above* this floor; nothing ever moves *out* of it without a human-authored
edit to this list.

## Cross-session transport: inbox + drain (T21)

The human is not the message bus. Tasks travel between roles through
tree-native inboxes — `.scratch/inbox/<role>.md`, untracked: **mail, not
memory** (anything that must persist goes in the queue, a thread, or a doc —
never only in an envelope).

- **Format:** header `AUTOPILOT: off|on` (default **off**; only the human
  flips it); append-only blocks `--- id: NNN` / `from:` / `serves: <queue-row
  fragment>` / optional `kind: task|design|fyi|draft` / optional `after:
  <condition>` / `status: WAITING|QUEUED|TAKEN|DONE|UNREAD|READ` / body.
- **Non-work kinds (T25):** `kind: fyi` — ephemeral context (schedules,
  intents, heads-ups): no `serves:` needed, the drain skips it BEFORE the
  orphan check (never ridden, never orphan-flagged), read at orientation →
  mark READ; READ fyis are pruned at the next session-end. `kind: draft` —
  non-expiring intake (discussion results, ad-hoc proposals): never ridden,
  never pruned; lives until a /coordinate triage verdict (promote |
  merge-into | park | drop).
- **Cross-agent write rule:** a role appends only to OTHERS' inboxes and
  updates only its OWN inbox's item statuses — the sole cross-agent write
  surface.
- **Drain** (`.claude/hooks/drain-inbox.sh`, Stop hook): with AUTOPILOT on,
  a session that finishes takes the next QUEUED item instead of stopping.
  Four hard guards: orphan refusal (no valid `serves:` row ⇒ skip + flag);
  design-envelope legalization (`kind: design` must instruct "log as thread
  stub before work proceeds" — ephemeral must carry durable's birth
  certificate, or it doesn't ride); drain budget (max 3 chained items —
  unattended chaining is new ground; the budget is the guardrail, not
  decoration); the never-auto floor above is untouched mid-drain. Malformed
  anything ⇒ normal stop.
- **Dispatcher** (`scripts/dispatch-inbox.sh`, host-side until T18): promotes
  `status: WAITING` items whose `after:` condition holds (`pr-merged #NN`,
  `row-done <fragment>`) to QUEUED.
- **Status single-sourcing:** inbox status = delivery state only; work state
  lives in the queue row, flipped in the shipping PR as today; an item isn't
  DONE until its row moved; the queue never references the inbox — canon must
  not depend on transport.
- **AUTOPILOT scope (T23, human-endorsed 2026-06-11):** the flag is per
  **role × project**. Per role because it is a trust grant ("this role may
  auto-take work") — trust profiles differ per role and role-scoped rollback
  needs independent switches; per project because it expresses trust in THIS
  repo's guardrail stack (deny rules, never-auto floor, gate, CODEOWNERS) —
  it lives in repo mail only, never in user-global config, and new projects
  start off with no inheritance. NOT per environment: one shared tree = one
  header; env differences belong in the READER (the LL-011 role guard), not
  the flag. Not per agent-harness yet — today role↔agent is 1:1; at the
  second harness this becomes per (role × agent), defaulting OFF for a new
  agent until its guardrail wiring is validated.
- **Kill-switch:** the presence of `.scratch/inbox/HALT` gates ALL roles'
  drains to no-op regardless of headers — "hard trigger reverts both roles"
  (docs/auto-trial.md) is a single atomic touch. The drain checks it before
  the AUTOPILOT gate.
- **Flip audit:** every AUTOPILOT header flip and HALT create/remove appends
  one line — `timestamp | actor | old→new | reason` — to
  `.scratch/inbox/flips.log` (untracked like the mail, but a trace): trust
  changes must not exist only in a chat transcript.

## Context economy (T25, human-blessed 2026-06-11)

- **State lives in artifacts; channels carry intent + work only.** Never
  mail what an artifact already says — link the queue row, thread, or doc.
- **As-of timestamps** on anything written-for-later; **ground truth is
  re-read at act time** (the whoami law generalized: a reader resolves
  reality when acting, never trusts a stale note).
- **One decision = one envelope.** Amendments append to the existing item;
  sibling-minting for the same decision is drift.
- **Gates are events** (`row-done <fragment>`, `pr-merged #NN`), never
  times; schedules live with the human (routines), not in inter-agent state.
- **New channels pay rent:** a proposal for a new context surface must name
  the failing channel and what is retired (per-role status boards stay
  deferred BY NAME until fyi proves insufficient).

## Coordinator authority (T25 — pinned to the HAT, not the skill)

- `/coordinate` is **propose-only, always**: no verdict self-disposes; the
  verdict batch PR is the proposal, disposal stays with the arming role.
  Arming = an advisor review act; frozen paths keep human admin-merge.
- **Scheduled runs are hat-holder only** (advisor today). Non-hat runs are
  allowed but their output is marked "non-hat run".
- **Self-verdict flags:** verdicts on items the runner authored, or that
  route work to/away from the runner, are flagged for the arm-er.
- **Drops always need cross-role ack** — the only verdict that destroys
  information.
- **Cadence:** one daily coordinator run under the advisor hat, rendered
  yesterday / today / blockers into the 08:00Z reminder slot; absorbs the
  trial daily audit during the trial week. No synchronous ceremony.
- **Coordinator as a separate agent stays deferred BY NAME** — promotion by
  evidence (volume or conflict-of-interest incidents; prerequisite T18).

## Work selection (T25 family — human-blessed 2026-06-11, transcribed from envelope 023)

**Work-source cascade** — the order any role finds its next work; the
coordinator *generates*, never assigns:

1. **Drain** — inbox cargo (QUEUED envelopes, drain rules above).
2. **Queue rows** — unblocked rows in docs/PLAN.md, dependency order.
3. **Queue dry?** The coordinator's DAILY run (never per-session, never an
   auto-`/replan` at session start) generates candidate rows from **spec
   gaps** — uncovered FRs, clauses without FRs, spec'd-but-unbuilt verbs
   (spec-map yellow/red), unmet phase criteria — and **proposes** rows
   (propose-only; the human disposes).
4. **Specs exhausted too?** Emit a **"PHASE SCOPE COMPLETE" report**:
   queue dry · coverage % · phase-criteria status · candidate next-scope
   menu. Never "project done" — phase/charter boundaries are human
   sentences only.

**Work-mix balance — principles, not rules** (the human's experiment,
judgment-first; the boundary doctrine: *rules for safety* — irreversible —
*principles for optimization* — drift-visible, reversible. This trial tests
the principle side only; the deny floor and never-auto floor are untouched):

- p1. The product is the specs made real; harness work exists to serve that.
- p2. Among unblocked work, prefer what moves a phase criterion or spec/FR
  coverage; choose meta work when it unblocks or de-risks product work.
- p3. Low-hanging fruit is a tiebreak, never a diet.
- p4. Human-priority jumps are legitimate and SIGNED (named in the row).
- p5. Report the mix honestly; skew is a signal to surface, not hide.

**Telemetry (mandatory — judgment's audit trail):** the daily standup
carries `work mix: N% product-spec / M% meta; phase criteria last touched
D days ago`; the weekly report graphs the history.

**Dormant mechanics (noted, NOT active — cheap switch-on, do not implement
early):** row class tags (`product-spec` | `meta`); a `/replan` invariant
"unblocked product-spec outranks unblocked meta; meta jumps only by signed
human priority"; triage parks meta drafts when the product band starves.
**Switch trigger (pre-agreed evidence event, no relitigation):** product-spec
share starving for TWO consecutive weekly reports without signed
human-priority causes ⇒ the mechanics flip ON.

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
