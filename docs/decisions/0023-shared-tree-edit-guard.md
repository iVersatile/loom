# Shared-tree edit-guard + own-worktree topology — ADR-0023 (adv-067 TASK 3)

**Status: Accepted (human, 2026-06-17).** Decision accepted; the CODE (edit-guard hook
+ FR + guard test; own-worktree session wiring) is now buildable — advisor-in-loom (T34)
**Phase 3 Slice C**, with the two human-gated parts (the PreToolUse hook + the own-worktree
launch shape) applied by the human per *Apply steps*. The amendment (own-worktree generalizes
per-seat→per-session for advisor + ephemeral authors in one loom-dev) is part of this acceptance.

> Originally drafted by loom-author as a decision-first proposal behind [[LL-015]] (spec before
> code, RULES §2/ADR-0006); accepted via the established "agent drafts, human accepts" pattern
> (cf. ADR-0015 §harness, ADR-0018).

## Problem (the recurrence, LL-015)
Both seats — the host advisor and the in-container Writer — edit code in `main`'s
single shared checkout (`/workspace/loom`). Twice (2026-06-13, 2026-06-16) they
collided on the same files; on 2026-06-16 an advisor `git stash` swept the
Writer's commingled adv-065 `container.go`/`build_test.go` out of `main`'s working
tree (recovered from the branch — no loss, but the in-flight edits vanished). The
mitigation ("work in a worktree, never edit `main`") lived only as a one-sided
memory note. RULES §5: guardrails are mechanism, not trust — an unenforced
convention is not a rule, and it must bind EVERY seat. Host `ps` cannot even see
the in-container Writer (separate pid namespace), so "one process" is not evidence
the tree is free.

## Decision — two layers, symmetric across both seats
1. **Own-worktree topology (the real fix).** Each seat operates in its OWN git
   worktree; `main`'s primary checkout holds NO agent CWD. The Writer launches
   into its own worktree; the advisor works in its own (it already does
   intermittently). Likely realized as session/container wiring (the launch lands
   the agent in a per-seat worktree) plus an advisor convention. TOPOLOGY.md +
   HARNESS.md document the resulting shape.
2. **Edit-guard hook (the backstop).** A `PreToolUse` guard that refuses `Write`/
   `Edit` (and equivalents) targeting paths under the project tree while `HEAD` is
   `main`/`master` — mirroring branch-guard's role for commits, extended from
   commits to EDITS. Symmetric: materialized onto BOTH harnesses (the playbook
   `harness:` set materializes the Writer's; the advisor's is applied to its
   harness). Mirror branch-guard's escapes (an explicit, audited override for the
   rare legitimate on-main edit). This is the seatbelt for when the topology is
   bypassed; the topology is what removes the hazard.
3. **Human-readable face.** A RULES/TEAM.md clause stating the rule (never instead
   of the mechanism). Draft clause:
   > **Shared-tree discipline (mechanized).** No agent edits files under a shared
   > checkout while `HEAD` is `main`/`master`; each seat works in its own git
   > worktree. Enforced by the PreToolUse edit-guard (both harnesses) — the
   > branch-guard analogue for edits. Override is explicit and audited.

## Why a hook AND a topology (not either alone)
The topology removes the collision source (no two seats in one tree). The hook is
the backstop for the gap the topology can't close by itself (a misconfigured
launch, a manual `cd` into `main`'s checkout). Branch-guard already proved the
pattern for commits; this extends the same mechanism-not-trust posture to the
edit that precedes the commit.

## FR + tests (follow-up, after acceptance)
- New FR (e.g. **FR-GUARD-EDIT-001**): "Write/Edit under the project tree while
  HEAD is main/master is refused by the edit-guard, both harnesses; the override
  is audited." Wired into `fr-verify`.
- Guard-suite test mirroring `internal/guard/block_test.go` /
  `internal/guard/drain_test.go`: on-main edit blocked · override allowed ·
  off-main edit allowed · path-scoping (only the project tree).

## Apply steps (path classes + required overrides)
| artifact | path | class | how it lands |
| --- | --- | --- | --- |
| ADR-0023 | `docs/decisions/0023-shared-tree-edit-guard.md` | FROZEN | human accepts via merge / `ALLOW_SPEC_CHANGE=1` |
| LL-015 | `docs/LESSONS_LEARNT.md` | normal | ships in this PR (committable) |
| edit-guard hook | `config/hooks/edit-guard` (+ harness PreToolUse wiring) | **TRUST** | human-applied / `ALLOW_TRUST_CHANGE=1` (029.C) |
| RULES/TEAM clause | `docs/RULES.md` or `docs/TEAM.md` | frozen-by-convention | human admin-merge = acceptance |
| FR + guard test | `docs/FR-registry.yml`, `internal/guard/*_test.go` | normal | Writer build, after the ADR is accepted |
| own-worktree wiring | session/container launch + TOPOLOGY/HARNESS docs | topology | design call (advisor + human) |

## Sequencing
1. Accept this ADR (the decision). 2. Human applies the edit-guard hook +
PreToolUse wiring (trust path) and decides the own-worktree launch shape.
3. Writer builds the FR + guard-suite test + doc clauses on top. Spec before code:
no hook code ships until the decision is on record.

## Amendment — advisor-in-loom (T34), 2026-06-17
The original framing was two seats across two containers (host advisor + in-container Writer). The **advisor-in-loom** workstream (T34) puts the persistent advisor **and** N ephemeral author-workers in ONE `loom-dev`. The own-worktree topology (layer 1) is the load-bearing enabler for that co-existence: **each session — the advisor and every ephemeral worker — operates in its OWN worktree; `loom-dev`'s primary checkout holds no agent CWD.** This is also ADR-0025's hard precondition for N>1 and the reaper's recovery unit. The edit-guard hook (layer 2) + the RULES clause apply unchanged, symmetric across all in-container sessions. No new decision — this records that "own-worktree per *seat*" generalizes to "own-worktree per *session*" once both roles share loom-dev.
