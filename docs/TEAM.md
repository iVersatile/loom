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

## Shared-tree git discipline (human-blessed 2026-06-12, transcribed from envelope 041/draft 028)

Four rules for the one tree two seats share. This clause is **frozen** —
human admin-merge is the acceptance; agents propose changes via PR.

1. **End-of-work: main, clean.** The Writer never leaves the tree parked on
   a branch — every work block ends `git checkout main` with a clean status
   (branch refs keep the work). Prevents stale-hook-stack execution (the
   HIGH-2 class: a session inheriting whatever hook set the parked branch
   carries) and the dirty-passenger hazard (uncommitted edits riding into
   the next session's first commit).
2. **Currency contract: main-is-current is the ADVISOR's deliverable**, at
   exactly two event types — the morning standup (pull/ff) and handoff-merge
   completion. The Writer *assumes* currency only at those events; at any
   other moment it verifies before depending on main (anonymous fetch works
   in-container). Nobody else moves shared refs casually (Shared-`.git`
   rule above).
3. **Hand off per-branch, not per-session.** Each built branch goes to the
   advisor inbox immediately when it's done — **as a `kind: git-task`** (the
   explicit git-controller handoff, below) so the relay is tracked, not
   detected — push → PR → merge while the diff is hours old, not at session end. A handoff that waits for the
   session boundary is a handoff that rots (see the 2026-06-12 cold-start
   loss class: in-flight branches unrelayed at orient,
   docs/e2e/cold-start-continuity.md).
4. **Resolver-as-tool.** Queue-table merge conflicts in docs/PLAN.md are
   resolved by the row-union resolver (`scripts/resolve-plan-union.py`),
   never by hand or by `HEAD`-wins — HEAD-wins silently reverts rows the
   branch never edited (caught live 2026-06-12, 8/8 rows resolved by the
   seed script). **Hard rule:** ALWAYS diff the resolved PLAN against
   `origin/main` before pushing.

Specimens (incidents the rules encode — keep with the clause):

- **Cross-boundary worktree prune (2026-06-12):** an in-container cleanup
  pruned host-side `.git/worktrees` metadata — the container cannot see the
  host's tmp worktree directories, so `git worktree prune` judged them
  dead. Never prune worktrees across the container/host boundary; prune
  only from the seat that created them (existing law, now with incident).
- **guard-bash flag-anchor misfire (2026-06-12, ×2):** the hook blocked a
  legitimate recursive tmp-dir delete because its pattern anchors on the
  slash after the flag, not the path root — and fired a second time on a
  draft *quoting* the first block. Evidence feeds the T29 segment-aware
  evaluation row (docs/threads: T29); until that lands, expect
  false-positives on quoted commands and re-phrase rather than weaken the
  pattern.

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
- trust-config writes (`.claude/settings*.json`, `.claude/hooks/**`, and the
  user-level equivalents under `~/.claude`) — permission changes are trust
  changes (029.C, envelope 040): agents propose diffs, never apply. Enforced
  git-side by protect-paths (`ALLOW_TRUST_CHANGE=1` audited override, human
  instruction only — deliberately separate from `ALLOW_SPEC_CHANGE`);
  harness-side Edit/Write deny rules are themselves a settings change and
  await human hands (queue row 029.B). `.claude/skills/**` is not trust
  config — skills are instructions the permission stack mediates, not the
  stack itself.
Prompt-volume work (allowlists, the compound allow-hook) may grow what's
*above* this floor; nothing ever moves *out* of it without a human-authored
edit to this list. (Additions are the stricter direction and arrive by PR —
the 2026-06-12 trust-config line landed that way, envelope 040.)

## Cross-session transport: inbox + drain (T21)

The human is not the message bus. Tasks travel between roles through
tree-native inboxes — `.scratch/inbox/<role>.md`, untracked: **mail, not
memory** (anything that must persist goes in the queue, a thread, or a doc —
never only in an envelope).

- **Format:** header `AUTOPILOT: off|on` (default **off**; only the human
  flips it); append-only blocks `--- id: NNN` / `from:` / `serves: <queue-row
  fragment>` / optional `kind: task|design|fyi|draft|git-task` / optional `after:
  <condition>` / optional `parked-on:`/`parked-at:`/`superseded-by:` (ADR-0020,
  before `status:`) / `status: WAITING|QUEUED|PARKED|TAKEN|DONE|UNREAD|READ` / body.
- **Non-work kinds (T25):** `kind: fyi` — ephemeral context (schedules,
  intents, heads-ups): no `serves:` needed, the drain skips it BEFORE the
  orphan check (never ridden, never orphan-flagged), read at orientation →
  mark READ; READ fyis are pruned at the next session-end. `kind: draft` —
  non-expiring intake (discussion results, ad-hoc proposals): never ridden,
  never pruned; lives until a /coordinate triage verdict (promote |
  merge-into | park | drop).
- **Git-controller handoff (`kind: git-task`):** an EXPLICIT, tracked handoff of
  a git action the emitting seat cannot perform itself — addressed to the
  git-controller (the advisor today, ADR-0017). Body names the **branch + the
  action** (`push+PR`, or `merge #N`) + the queue row it serves. The emitting
  seat does NOT rely on the controller *detecting* its branch; it flags the
  task. Not ridden by the Writer's drain — the advisor drains git-tasks as its
  git-controller routine (push → PR → relay), so the boundary is **visible and
  queue-tracked, not implicit**. **Scales down with capability:** once the
  Writer gains push + `gh pr create` (036/T15 clears, ADR-0017 §1) it performs
  those itself and emits `git-task` ONLY for `merge` — the permanent
  human/reviewer gate (ADR-0017 §2, no agent merges its own work to main).
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

## Autonomy closed-loop: PARK → pull-next → re-surface (ADR-0020)

A block must never halt-and-drop. When a delivered task can't proceed, the
Writer parks it (nothing lost), keeps working the next independent item, and the
drain re-surfaces the parked one when its blocker clears — closing the circle
without a human relay. The decision logic is two shared, non-trust-path scripts
(`config/hooks/resurface-decide`, `config/hooks/pull-next`) the trust-path drain
sources (the drain diff itself is human-applied). Guard-tested:
`internal/guard/resurface_test.go`.

- **PARK fields (HEADER — MUST precede `status:`).** The drain parser reads
  header fields only before `status:`; a field after it is silently missed, so a
  malformed park fails closed (see below). Fields:
  - `parked-on: <predicate>` — the blocker. **Fixed vocabulary, never eval'd**
    (ADR-0020/R2, same doctrine as the wake-keystroke constant): `exists:<path>`
    | `pr-merged:<n>` | `item-status:<id>=<STATUS>`. Anything else = fail-closed
    (stay parked). **Ranked external-truth first** (R7): prefer `pr-merged:`
    (human-merged git truth) over `item-status:` (agent-writable inbox);
    `item-status:` is acceptable only because re-surface causes a bounded *turn*,
    never an action.
  - `parked-at: <epoch>` — when parked; drives the over-age tier.
  - `superseded-by: <id>` — supersede-skip.
- **Drain decisions** (`resurface-decide`): DELIVER (QUEUED) · SKIP-PARKED (dep
  uncleared) · RESURFACE→QUEUED (dep cleared) · SKIP-SUPERSEDED · **ESCALATE**
  (dep uncleared AND `now − parked-at > LOOM_MAX_PARK_AGE`, default 7d — surface
  to a human; **never auto-drop**, losing work is worse than a stale park; R4) ·
  ESCALATE (malformed: PARKED with no parseable `parked-on`).
- **Park-on-block behavior (operating instruction).** A blocked delivered task:
  (1) commit WIP to the task branch (nothing lost); (2) set `status: PARKED` +
  `parked-on:` + `parked-at:` (header order); (3) escalate the blocker to the
  advisor inbox; (4) **pull the next independent eligible item** and keep
  building — DO NOT HALT. The picker (`pull-next`) takes the first QUEUED item
  whose `depends-on:` is empty or DONE, SKIPPING any that depend on a
  PARKED/QUEUED item (no priority inversion, R5); all-blocked ⇒ stay idle (the
  poll re-surfaces / the over-age tier escalates).

## Backlog-readiness layer: which PLAN row is auto-pullable (ADR-0022)

ADR-0020 closes the loop over the INBOX; ADR-0022 extends it to the PLAN/FR
backlog so the loop can self-refill instead of waiting on a human triage gate.
**Slice 1 (offline, here):** `scripts/readiness-decide` reads the tactical-queue
fence + an EXTERNAL-TRUTH merged-refs file and prints, per `queued` candidate
row, ONE fixed-vocab verdict — `READY` (the single highest-priority eligible
row) · `BLOCKED-DEPS` · `NOT-EXEC-READY` · `INVERSION` · `SUPERSEDED`. Pure
read-only mechanism — no promotion, no spawn (those are slices 2–5).

- **`[class:exec]` row-class tag = the EXEC-READY gate (ADR-0022 Decision 3b).**
  A row is exec-ready ONLY if it carries the exact literal `[class:exec]` tag.
  **Absent ⇒ `NOT-EXEC-READY`** — fail-closed: a row opts INTO autonomy
  explicitly; design-stubs / thread-incubators / un-triaged rows are never
  exec-ready. This is a STRUCTURED tag, deliberately NOT the drain's `/thread
  stub/` substring heuristic (trivially evaded). Tagging a row is a triage act —
  slice 1 ships only the reader, so today every real row reads `NOT-EXEC-READY`.
  (Exec-readiness is DISTINCT from the self-selection floor below: the tag gets a
  row to `READY`; the allow-list decides whether `READY` auto-QUEUEs.)
- **Deps by EXTERNAL TRUTH only (amendment 2).** `depends-on` PR refs (`#NNN`)
  must all appear in the host/CI-written merged-refs file (`gh pr view`-derived).
  The runner reads NOTHING else — **never** git commit subjects
  (`drain-inbox.sh:45`), which the Writer forges by wording a commit. A row that
  merely *claims* "merged (#150)" in prose, with 150 absent from external truth,
  is `BLOCKED-DEPS`. A non-empty `depends-on` with no resolvable `#NNN` (textual
  dep) is also `BLOCKED-DEPS` (conservative: fail to NOT-pull).
- **No inversion (R5, reused).** Among eligible rows, only the first in queue
  order is `READY`; the rest are `INVERSION` until it clears.
- **Injection-proof.** Verdicts are hardcoded constants; row text is read with
  awk string ops only — never shelled, never echoed into a decision token (only
  an integer row index prints).

**Slice 2 (offline, here):** `scripts/promote-next` ACTS on the single `READY`
verdict by minting a real, parser-valid inbox envelope (`--- id: promote-<slug>`)
for that row — the backlog→inbox REFILL. It reuses `readiness-decide` for the
verdict (one home for the security logic), so a `BLOCKED-DEPS`/`NOT-EXEC-READY`/
`INVERSION`/`SUPERSEDED` row never mints. It is NOT a deliver-direct path — the
existing drain delivers the minted envelope, audit trail intact.

- **Self-selection floor = the auto-pullable row-class allow-list (amendment 5),
  DISTINCT from the never-auto PERMISSION floor (T23, untouched).** A `READY`
  row's class must be on a pre-declared allow-list (`LOOM_AUTOPULL_CLASSES`,
  **default empty**) to mint `status: QUEUED`; off the list it mints `status:
  CONFIRM-REQUIRED` — the human tier (the drain only DELIVERs `QUEUED`, so it sits
  inert until a human flips it). Default-empty = nothing self-selects without an
  explicit opt-in. This governs *which work self-selects*; permission is a
  separate gate.
- **Full-key `serves` (M7 fix).** The minted `serves:` is the row's EXACT serves
  cell; a sibling row whose serves is a *substring* is never conflated — no
  substring orphan-match (`drain-inbox.sh:62`).
- **Idempotent.** The id is `promote-<slug>` (sanitized `[a-z0-9-]` from the row
  title); a re-run finds the existing id and no-ops — no double-mint.
- **Rehydration-poisoning mitigation (S2).** The minted body is a FIXED template
  referencing the row only by its sanitized slug — backlog prose is DATA, never
  echoed as a trailing instruction.

**Slice 3 (offline, here):** `scripts/spawn-guard` is the "may I spawn now?"
decision the future loop (slice 4 actuator, gated on ADR-0019 PR4) calls BEFORE
every spawn. The drain's 3-per-cycle budget is `stop_hook_active`-scoped =
intra-process; a freshly *spawned* worker resets it, so it does NOT bound work
across spawns. spawn-guard owns the DURABLE cross-spawn bound (`.drain-count` is
explicitly not it). Fixed-vocab verdict:

- **`DENY-HALT` checked FIRST** (before the rate test), mirroring the drain's
  HALT-before-AUTOPILOT order (T23): a HALTed system spawns nothing and pays no
  spawn cost.
- **`DENY-RATE`** when in-window spawns ≥ max, reporting bounded backoff (~seconds
  until the oldest in-window entry ages out). Max + window are params
  (`LOOM_SPAWN_MAX` default 3, `LOOM_SPAWN_WINDOW` default 3600s).
- **`ALLOW`** records the grant by appending one epoch line to the ledger
  (`.scratch/inbox/.spawn-log`) — the ledger is the single source of the count.
- **Fail-closed:** a malformed/unreadable ledger ⇒ `DENY-RATE` (treated at-limit),
  never fail-open. A missing ledger is the legitimate first spawn (ALLOW).
- **Injection-proof:** verdicts are constants; a non-epoch ledger line fails closed
  and reaches no shell.
- **Carry-forward (slice-5 hardening, human-applied):** the ledger is agent-
  writable `.scratch/` — an agent could delete it to reset the count. Before the
  spawner is load-bearing the ledger must be HOST/append-only/tamper-evident (same
  "external truth has two halves" class as merged-refs + the autopull allow-list).

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

## Phase-close review gate (P7 — human-decided 2026-06-11)

A phase closes only through the mandatory independent review
(docs/review-gate.md): three dimensions (security, architecture,
harness-health), each graded by a fresh context that **authored nothing in
the phase** (excludes Writer and advisor authoring contexts). Severity rubric
is fixed before the review: Critical = no waiver, the phase stays open; High
= fix or written human risk-acceptance; Medium/Low → backlog. Findings name
the WHAT, never the HOW. What a reviewer hand-checks once, doctor mechanizes
next (the review's mechanization note feeds the doctor checklist). Precedent:
docs/reviews/phase-1-review.md (executed 2026-06-12). The close edit itself
stays human — no agent self-approves a phase completion.

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
