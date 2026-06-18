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
  │  │  ~/.gitconfig ENGINE-LIVE (T16 PR 3: `gitconfig` dotfiles│  │
  │  │   ref → ~/.gitconfig; base playbook declares the noreply│  │
  │  │   identity per docs/TEAM.md; doctor host:gitconfig      │  │
  │  │   verifies completeness; goes plain LIVE at the next    │  │
  │  │   host `loom build`)                                    │  │
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
  role-push-guard                DEFENSE-      git push / gh for any non-advisor
   (deny-override on the          until-D       session (advisor-in-loom T34,
   union push allow)                            Slice A) — see note below
  container egress restriction   MISSING-T20   allowed go test/go build run
   (networking: or proxy                       arbitrary code incl. network
   sidecar; in-container                       I/O — harness rules cannot
   iptables rejected while root)               see it
```

Full-auto clearance is event-gated on this stack completing: the queue row
"re-run auto-mode evaluation" unblocks when T16 hooks + T10 non-root + T20
land. The row is the schedule.

**role-push-guard is DEFENSE-IN-DEPTH, not a guarantee, until Slice D.** In a
multi-role-per-container loom-dev (ADR-0021 Option A), the advisor and the
ephemeral author fleet share one `~/.claude/settings.json` carrying the UNION
push allow (`git push`, `gh pr …`). That allow is role-blind — the engine
consults it BEFORE any role logic — so the narrowing is `role-push-guard`, a
PreToolUse deny-override that blocks `git push` + `gh` for any session whose
launch-bound `LOOM_SESSION_ROLE` (root-marker fallback) is not `loom-advisor`
(deny beats allow). A running session cannot mutate what its own hook sees
(confer Q2, empirically confirmed). The SOLE residual hole is spawn-time
re-exec — an author running `LOOM_SESSION_ROLE=loom-advisor claude -p "push"`
starts a NEW advisor-env session that bypasses this hook. Closing it requires
**Slice D** — `spawn-guard`, a sibling PreToolUse deny-override that blocks a
`claude` spawn for any non-advisor session (same launch-bound role check, exit 2).
Once spawn-guard lands + is validated in-container, role-push-guard's residual is
closed and "authors cannot push" becomes a guarantee. Until then it is FALSE; no
doc/ADR may claim it as settled.

**INVARIANT — a role-scoped capability is a union ALLOW *plus* a role DENY-hook,
and they are CO-DEPENDENT.** The pattern (A1/role-push-guard, D1/spawn-guard) is:
the command sits in `permissions.allow` for ALL sessions (role-blind), and a
PreToolUse deny-override (`exit 2`, launch-bound role) narrows it to the privileged
role. **The command MUST be in the allow** — if it is not:
  1. the **approval gate masks the hook** — a not-in-allow command halts at
     "requires approval" *before* the deny surfaces, so you can neither validate
     nor rely on the deny (this is the LL-016 discriminator trap — it bit the
     `gh auth status` *and* the D1 `claude --version` tests); and
  2. the **privileged role cannot act headlessly** — its own call also prompts.
So `Bash(claude:*)` is in the union allow alongside the deny: the advisor spawns
headlessly, spawn-guard blocks the author despite the allow (deny beats allow).
A deny-hook whose command is absent from the allow is untestable and inert in
headless mode — never ship one without its allow.

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
