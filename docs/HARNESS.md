# HARNESS — how the agent harness is wired (synthesis)

A living overview of the harness-home design: the diagrams, the layers, and
what is live vs pending vs missing. **This document decides nothing** — every
section cites its source of truth; if this page and a cited source disagree,
the source wins (RULES §2 spirit). Keep it current the same way the tactical
queue stays current: a PR that changes the harness updates its marker here in
that same PR.

Status markers: **LIVE** (verifiable today — most have a claims-script probe)
· **PENDING-T16** (decided in ADR-0015, engine work not built) ·
**MISSING-T20** (known gap, option space captured, no decision yet).

## Who owns what (the five home places)

| Piece of the design | Owning document | Nature |
| --- | --- | --- |
| The config/state decision + rationale | docs/decisions/0015-harness-home-config-vs-state.md | frozen, append-only (Accepted) |
| safe-auto, permission stack, write discipline, merge policy | docs/TEAM.md | working convention |
| History + open edges (T16/T18/T19/T20 evolution) | docs/OPEN-THREADS.md | append-only record |
| Operating context — who operates, where (mac-dev / windows-dev / ai-user) | docs/TOPOLOGY.md | reference |
| What is *verifiably* live right now | scripts/verify-loom-dev.sh | executable claims (predict → verify) |

This page is the sixth: the synthesis. It duplicates no decision — only the
shape.

## 1. The core principle: one seam, cut by mutability (ADR-0015)

Declared config flows into `~/.claude` on every build; mutable state accretes
there and loom never touches it.

```
  config source (repo: config/)          ~/.loom/home (host staging)
  ┌──────────────────────────┐  materialize  ┌──────────────────┐
  │ playbook.yml             │ ────────────► │ .claude/         │
  │  dotfiles: / harness:*   │ writeIfChanged│   settings.json  │
  │ dotfiles/claude/...      │  (+x for .sh) │   statusline.sh  │
  │ hooks/ skills/* gitconfig*│              │ .bashrc.d/...    │
  └──────────────────────────┘               └────────┬─────────┘
       versioned · reviewable · re-converged           │ docker cp
                                                       │ (gated by the HOME
                                                       ▼  digest sentinel, T7)
  ┌─────────────────────────── container ──────────────────────────┐
  │  ~/.claude  ◄── mounted volume: <container>-claude  (T14) LIVE │
  │  ┌─────────────────────────┬────────────────────────────────┐  │
  │  │ DECLARED CONFIG         │ MUTABLE STATE                  │  │
  │  │ loom re-writes each     │ loom NEVER touches             │  │
  │  │ build; drift erased     │                                │  │
  │  │  settings.json     LIVE │  .credentials.json (OAuth) LIVE│  │
  │  │  statusline.sh     LIVE │  settings.local.json       LIVE│  │
  │  │  hooks/         LIVE*   │  projects/<p>/memory/      LIVE│  │
  │  │  skills/  ENGINE-LIVE*  │  session history           LIVE│  │
  │  │  ~/.gitconfig PENDING-T16 (must declare the noreply     │  │
  │  │   identity — docs/TEAM.md commit-identity rule; ships   │  │
  │  │   as a dotfiles: ref, T16 PR 3)                         │  │
  │  └─────────────────────────┴────────────────────────────────┘  │
  │  volume survives --force/teardown; wiped only by the opt-in    │
  │  --clean-state tier (ADR-0014 addendum)                        │
  └────────────────────────────────────────────────────────────────┘
  * harness: schema + materialize handlers = LIVE in the engine
    (T16 PR 1: FR-BUILD-009, FR-SCHEMA-008; rides the dotfiles staging
    pipeline, T7 digest covers it). The base playbook declares harness:
    since T16 PR 2 — settings.json (hook registrations + permission
    floor, harness-owned, off the dotfiles: list) and hooks/guard-bash
    are plain LIVE; verify-loom-dev asserts both as PRESERVE claims.
    skills/ stays ENGINE-LIVE: the handler works, no env-wide skill is
    declared yet (project skills live in the repo tier below).
    session-snapshot stays undeclared — content design parked
    (judgment-trial C4).
```

Why the split is where it is: a converge build erased harness-written runtime
state from `settings.json` (the live evidence in ADR-0015's context) — one
file cannot be both declared config and mutable state. So: `settings.json` is
config, `settings.local.json` is state.

## 2. Two-tier policy ownership (ADR-0004 applied; ADR-0015 decision 4)

```
  BASE PLAYBOOK (env-wide → materialized into ~/.claude)    LIVE (PR 2)*
  │  guard hooks(LIVE) · base deny rules(LIVE) · statusline(LIVE)
  │  · git identity PENDING-T16 (PR 3)
  │  declared via harness: — explicit-by-reference, like rules:
  ▼
  PROJECT REPO (/workspace/<project>/.claude/ — tracked)    LIVE
     settings.json   allowlist + deny floor + defaultMode: acceptEdits
     agents/         investigator · test-runner
     skills/replan/  Coordinator-hat queue audit
     zero engine involvement — the working tree IS the mount, so it
     survives rebuilds for free; loom may later assert presence
     (doctor), never author it
```

## 3. The permission stack (safe-auto — docs/TEAM.md)

```
  layer                          status        stops
  ─────────────────────────────  ────────────  ──────────────────────────────
  deny floor (ALL modes,         LIVE          creds reads · curl/wget/
   even bypass)                                WebFetch/WebSearch · sudo ·
                                               force-push · --no-verify ·
                                               in-container loom teardown /
                                               build --force
  allowlist + acceptEdits        LIVE          prompt friction without
   = "safe-auto"; bypass BANNED                blanket trust
  repo gate hooks (.githooks:    LIVE          commits on main · frozen-path
   protect-paths, branch-guard,                edits · failing/leaky commits
   make gate)
  guard hooks in ~/.claude       LIVE (PR 2)   the semantic layer — intent,
   (guard-bash; session hooks                  not just command names
   parked, judgment-trial C4)
  container egress restriction   MISSING-T20   allowed go test/go build run
   (networking: or proxy                       arbitrary code incl. network
   sidecar; in-container                       I/O — harness rules cannot
   iptables rejected while root)               see it
```

Full-auto clearance is event-gated on this stack completing: the queue row
"re-run auto-mode evaluation" unblocks when T16 hooks + T10 non-root + T20
land. The row is the schedule.

## 4. Convergence mechanics (ADR-0011 pattern, both surfaces — LIVE)

```
  build ─► exists? ─► compare TWO in-container sentinels:
                        /var/lib/loom/provisioned  toolset+agent digest
                        /var/lib/loom/home         staging-tree digest (T7)
           home stale  → docker cp + rewrite home sentinel   (never apt)
           tools stale → cp + re-run idempotent provision
           both fresh  → "exists" — a truthful no-op
```

Presence ≠ convergence holds for tools *and* `$HOME`; status strings are
truthful by construction (T7 resolution).

## 5. Session continuity (ADR-0015 decision 5 — PENDING-T16)

Today: a hand-carried handoff doc in `.scratch/` (untracked) + the in-tree
durable memory (OPEN-THREADS, the tactical queue). Decided target: a
session-end snapshot hook + session-start orientation as first-class
`harness:` items — the handoff convention retires when they land. Memory
seeds empty (no host import); harness-native memory under
`~/.claude/projects/` rides the volume and is designed behavior, not a
surprise.

## Edges that are someone else's thread

- Non-`~/.claude` agent state (gh/VCS credentials, `~/.config/gh`) — the
  T15-successor credential ADR (secret store + per-use helper; no secret at
  rest).
- Per-agent homes (gemini/codex) and per-project `settings.json` overlays —
  ADR-0015's revisit-if clauses.
- The Makefile↔playbook gate-dep joint check — T19's promote-to, design
  recorded, not built.
