# Prepared patch — ADR-0022 slice-5 HOST hardening (make the autonomy substrate load-bearing)

**Host + trust change → HUMAN-applied.** The substrate's consumer-side integrity
checks are built and default-OFF (`LOOM_EXTERNAL_TRUTH_STRICT` unset ⇒ the
verified loop keeps working). This patch is the **producer/host half** that lets
the operator flip them ON: host-own the external-truth files so the agent cannot
forge them, move the decision logic behind protect-paths so it cannot be rewritten
one level up, then flip STRICT. Until this lands the loop runs in its current
verified-but-not-tamper-resistant mode; nothing here changes agent behavior on
its own.

> Status: **prepared by advisor (loom-advisor), for human apply.** PLAN row
> "slice-5 HOST hardening". Pairs with the autopull Shape-1 trial (the trial can
> run BEFORE this — Shape 1 is supervised + author-can't-reach-main, so tamper-
> resistance is not its blocker; this hardening is the Shape-2 / unattended
> precondition).

## The three sub-tasks

### 1. Host-own the external-truth files (agent cannot rewrite/reset)

The STRICT checks (`scripts/readiness-decide:81`, `scripts/spawn-guard:69`) refuse
to trust an external-truth file the running (agent, uid 1000) process can WRITE —
because a forgeable source defeats the "external truth has two halves" doctrine
one level up (rewrite the deps file, not the script). The files:

| File | Producer | Consumer | Target ownership |
|---|---|---|---|
| `.scratch/inbox/.merged-prs` (merged-refs) | **host/CI** (`gh pr view <n> --json state,mergedAt`, one PR# per line) | readiness-decide, promote-next, self-wake-tick | host/root-owned **0644** — agent reads, cannot write ✓ clean |
| spawn-ledger (one epoch per granted spawn) | **agent** (`spawn-guard` appends a line on each grant — `spawn-guard:95`) | spawn-guard rate-check | **append-only** (`chattr +a`) — see ⚠ below |
| `.scratch/inbox/.wake-ticks` | **agent** (`self-wake-tick`) | self-wake-tick rate/window | append-only (`chattr +a`) — same ⚠ |

For the merged-refs file the producer is the host, so it is cleanly host-owned
read-only and STRICT passes as designed. **Replace the current drain-derived
`.merged-prs` (built from commit subjects — agent-controllable, the S1 red-team
hole) with a host/CI-written file** sourced from `gh pr view`, e.g. a small host
job:

```sh
# host-side, periodic — writes the merged-refs external truth the agent only reads
gh pr list --repo <owner/repo> --state merged --limit 200 --json number \
  --jq '.[].number' > /path/to/loom/.scratch/inbox/.merged-prs
chown root:root /path/to/loom/.scratch/inbox/.merged-prs
chmod 0644       /path/to/loom/.scratch/inbox/.merged-prs   # agent reads, cannot write
```

> ### ⚠ OPEN DECISION (must resolve before flipping STRICT for the ledger/ticks)
> The ledger and `.wake-ticks` are **agent-appended** (the agent IS their
> producer), yet the STRICT check fail-closes on any file the agent can write
> (`[ -w "$LEDGER" ]`). An append-only file (`chattr +a`) still has the write
> permission bit set, so `[ -w ]` is **true** for it — meaning current STRICT +
> `chattr +a` ledger ⇒ permanent `DENY-RATE`. So "host-own read-only" and "agent
> appends grants" genuinely conflict for these two files. Options, with a lean:
> 1. **(lean) Refine the STRICT predicate** to test *rewritability* (can the
>    agent truncate/unlink?) rather than the `-w` bit — e.g. accept a `chattr +a`
>    append-only file (the agent can add a grant line but cannot reset the count,
>    which is exactly the integrity property we want). Small, well-scoped change to
>    `spawn-guard`/`self-wake-tick`; keeps the producer = agent.
> 2. **Split producer/consumer** — the agent writes grant *requests* to a spool;
>    a host process appends to the real (host-owned read-only) ledger. Strongest
>    integrity, most host machinery.
> 3. **Scope STRICT to merged-refs only** (the one cleanly host-produced file);
>    keep the ledger on a separate tamper-evidence mechanism. Smallest, but leaves
>    the count-reset hole partly open.
>
> This is a **confer candidate** (advisor + author) before the build — do NOT
> silently pick one. The merged-refs half (sub-task 1, clean) can land and have
> STRICT flipped for `readiness-decide`/`promote-next` independently of this.

### 2. Relocate the substrate decision-scripts under `config/hooks/` (protect-paths)

The decision logic must move behind the trust class (protect-paths now covers
`config/hooks/**` — patch 0021) so amendment-2's external-truth guarantee is not
forgeable by rewriting the *script* instead of the *source*. Scripts to relocate:

```
scripts/readiness-decide   scripts/promote-next   scripts/spawn-guard
scripts/spawn-loop         scripts/self-wake-tick scripts/cold-check
scripts/cold-floor-cron
```

Two parts:
- **(agent-side, normal commit):** repoint every caller to the new paths
  (`config/hooks/self-wake-tick`, etc.) — the `/self-wake` skill, any cross-script
  references, docs. This can land first under the current guards.
- **(human/trust act):** the `git mv scripts/<x> config/hooks/<x>` itself — adding
  files into `config/hooks/**` is a protect-paths edit, so the move commit needs
  an audited `ALLOW_TRUST_CHANGE=1`. After it lands, the decision logic is
  commit-time-protected (weakening it requires the same audited floor).

```sh
# human hands, with the caller-repoint already merged or staged together:
ALLOW_TRUST_CHANGE=1 git commit -m "refactor(hooks): relocate autonomy substrate scripts under config/hooks/ (protect-paths — slice 5)"
```

### 3. Flip `LOOM_EXTERNAL_TRUTH_STRICT=1` in the loop seat

**Only after sub-task 1 lands** (else the loop fails closed: every queued
candidate → `BLOCKED-DEPS`, every spawn → `DENY-RATE`). Set it in the loom-author
loop seat's environment (the seat that runs the substrate). For the ledger/ticks,
gate this flip on the ⚠ decision above; for the merged-refs consumers it can flip
as soon as the host `.merged-prs` is in place.

## Sequencing (recommended)
1. Land the host `.merged-prs` producer (sub-task 1, merged-refs half) — clean,
   no open question.
2. Flip STRICT for the merged-refs consumers; observe the loop stays green.
3. Resolve the ⚠ ledger/ticks decision (confer) → small follow-up if option 1.
4. Relocate scripts under `config/hooks/` (sub-task 2) — trust act.
5. Flip STRICT fully (sub-task 3).

Each step is independently revertible (unset STRICT / `git mv` back); none is a
one-way door. The autopull Shape-1 trial does **not** depend on this — run it
first for the live signal, harden for Shape 2.

## Pointers
ADR-0022 (amendments 2 "two halves" / 3) · `scripts/readiness-decide:66-83`
(STRICT merged-refs check) · `scripts/spawn-guard:34-72,95` (STRICT ledger check +
the grant append that creates the ⚠ tension) · `scripts/self-wake-tick:20-34` ·
`scripts/promote-next:40-44` · patch `0021-protect-config-hooks.md` (the
`config/hooks/**` trust class this relies on) · PLAN row "slice-5 HOST hardening".
