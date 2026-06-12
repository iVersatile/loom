# PRE-REGISTERED — live-build-under-session e2e (run 1)
Registered 2026-06-12 ~10:50Z BEFORE execution. Human-blessed. Subject =
loom-author (UNANNOUNCED — observability question is only clean if the
subject isn't primed). Family: P2/e2e + the PLAN "agent-initiated
container lifecycle / task continuity" open item. NOT a judgment trial
(no cadence conflict). Lands in trial week ⇒ any misfire counts in the
day-2 ledger row.

## Hypotheses (scored pass/fail against observation)
H-A. Plain `loom build` converges in-place — Writer's exec session
     survives (no restart, no task-context loss).
H-B. Hook-script replacement under a live session is benign (no drain
     misfire, no guard-bash breakage mid-session).
H-C. The logs (.loom/actions.log structured + ADR-0010 diagnostic +
     build --json) let a third party reconstruct WHAT changed under the
     session, AFTER the fact, without having watched. (R1 predicts a
     gap here — a NEGATIVE result is the valuable finding.)

## Preconditions (all must hold before the button)
Two distinct moments (discovered 2026-06-12 mid-setup): GoodBinary-ready
(human can re-run plan to SEE F2 fixed, still holds build) vs GO (live
window open, build runs). Don't conflate them.

PREREQ CHAIN (dependency order; owner; gate):
1. [ADVISOR] Item 024 (C1+H2-H4+F1+F2) MERGED to main — else build
   materializes untested guardrail WIP; also makes plan honest re F2.
   Gate: PR #92 green (gate/integration/fr-verify). STATUS: #92 armed.
2. [ADVISOR] GOODBINARY — rebuild BOTH bin/loom + bin/loom-darwin-arm64
   from post-024 main (via main-pinned host worktree, NEVER from a live
   Writer checkout — specimen #3 today), grep-verify new strings.
   → ping human "GoodBinary ready". Blocked on (1).
3. [HUMAN] Honest dry-run: `plan --json` from Mac repo root now SURFACES
   the harness delta (was silent pre-024 = the ⑦/F2 before-exhibit).
   That delta = F2-fixed proof AND the no-distractions pre-stage.
   Blocked on (2). HOLD build here.
4. [WRITER — needs restart] A LIVE Writer session, mid a DOCS-ONLY task
   (021/022/023). THE experiment is build-UNDER-a-live-session; Writer
   stopped pending push ⇒ no subject until it restarts. Docs task keeps
   tree config ≈ main (variable = live+build, not WIP+build). Watcher
   b7mx1exji fires on 022/023 TAKEN. NOT MET while Writer is stopped.
5. [HUMAN+ADVISOR] HALT decision at go-time: drain armed (observe real
   behavior) or pre-touched HALT (drain frozen during run). Default:
   armed, since drain-misfire-under-build is itself in scope (H-B).
6. [ADVISOR] Capture BEFORE probes (below) → ping human explicit "GO".
   Human runs build from Mac repo root. Blocked on (4)+(5).

NOTE: prereqs (1)(2)(3) are the GoodBinary track (imminent); (4)(5)(6)
are the GO track (waits on a Writer restart onto a docs task). They are
NOT the same ping.

## Probes (advisor runs from devenv; human runs build from Mac)
BEFORE: verify-loom-dev output; sha256 of /root/.claude/settings.json +
each hook; tail .loom/actions.log; Writer's current task/inbox state.
DURING: capture `build --json` (stdout) + stderr (build log path);
wall-clock.
AFTER: re-hash the four files (diff vs before); actions.log new entries;
diagnostic log content; Writer writes an fyi describing anything it
NOTICED — or did not (not-noticing is itself a finding on observability).

## Abort / containment (pre-agreed)
Never-S1 by construction: no creds, no egress in play. Worst case =
drain/hook misfire = S3, contained by touching HALT. Reversal = re-run
build (idempotent) or restore hooks from git. Container survives
(in-place reconcile, no restart).

## Scope cut
Run 1 = clean-ish tree only (docs-only WIP). Build-against-engine-WIP is
a separate run 2 with explicit risk acceptance — not a freebie.

## Score → mechanization
Run-1 rubric becomes the spec for a verify-loom-dev / doctor claim:
"build under a live exec session leaves the session intact AND emits a
complete audit delta." Hand-check once (here), doctor inherits.

## BEFORE-EXHIBIT (pre-024 plan, captured 2026-06-12 ~11:0xZ, human Mac-run)
`loom-darwin-arm64 plan --json` (binary from b802556, PRE-024/F2):
  create=[] install=[] remove=[] noop=[git jq ripgrep gitleaks
  golangci-lint go@1.26 gopls make]
Reading: tools dimension CONVERGED + container up; plan SILENT on the two
harness PRESERVE claims = finding ⑦/F2 reproduced live (plan grades tools
only). PREDICTION: post-024 plan should SURFACE the harness delta (non-empty
or a new dimension) — that delta is the pass-signal that F2 is fixed, and
the honest pre-stage for the build run.

## BEFORE-SNAPSHOT (captured 2026-06-12 ~12:0xZ, advisor/devenv-side)
Trigger state: #92 merged (db6191c); GoodBinary built from it; Writer is on
021+022 (TAKEN). CORRECTION: .drain-count absent ⇒ claimed DIRECTLY (manual/
orientation), NOT via the autopilot drain — the drain STILL has not auto-fired
(more T27 evidence: drain silent even with eligible work). Tree clean on main;
docs tasks don't touch build inputs.

PLAN (human Mac-run, post-024 — the honest pre-stage, F2 confirmed):
  create: [lockfile loom.lock, dotfile ~/.claude/settings.json,
           dotfile ~/.claude/hooks/guard-bash]   ← settings+guard-bash = C1
  noop:   8 tools + agent:claude-code             ← F2 agent dimension
So build WILL: rewrite lock + materialize C1's settings.json + guard-bash
hook into the live container home (first time). Non-cosmetic; doubles as the
C1 end-to-end verification.

AUDIT LOG before: .loom/actions.log last entry 2026-06-10T22:24:07Z
(exec make gate). Build must append: lock.write + the harness materialize +
container.reconcile. H-C TEST = do those entries let us reconstruct the
change without having watched?
SOURCE hashes (what SHOULD land): settings.json 00bddbe0… ·
guard-bash 7d611010…
LOCK base_image before: debian:bookworm-slim@sha256:0104b334…

AFTER (advisor reads from devenv): re-plan (expect all-noop=converged) +
.loom/actions.log NEW entries (the H-C probe) + Writer reports whether its
LIVE session noticed the guard-bash hook replaced under it (H-B).

## RESULT — 2 runs, scored (2026-06-12 ~12:06Z)
RUN 1 (12:02:16Z, build under live Writer session): converged, lock_written
=true, "4 materialized". Audit log: lock.write + materialize settings.json
(written) + materialize guard-bash (written) + container.reconcile + lock.write
(repin). Lock re-pinned base digest 0104b334→96e378d7. docker cp .loom/home/.
RUN 2 (12:05:49Z, no-change re-run): converged, lock_written=FALSE, "4
materialized". Audit log: ZERO new entries. Diagnostic log: header only, NO
docker cp. => wrote nothing.

SCORE:
- H-A (in-place convergence, session survives) = PASS. Reconcile not recreate,
  twice; run 2 wrote nothing = genuine idempotency. (Writer session-survival
  confirm = H-B, pending.)
- H-B (hook swap under live session benign) = PENDING Writer report: run 1
  replaced ~/.claude/hooks/guard-bash under the live session. Need: did any
  bash call break / drain misfire / did the session notice?
- H-C (logs let you reconstruct) = SPLIT VERDICT:
  * STRUCTURED actions.log = RELIABLE: logged exactly the 2 real writes on
    run 1, ZERO on run 2 — you CAN tell a real change from a no-op. PASS.
  * HUMAN SUMMARY "N materialized" = UNRELIABLE: prints "4 materialized" on
    a 0-write run; counts declared-considered, not written; can't reconcile
    to the audit (4 vs 2-named run1, 4 vs 0 run2). FAIL — R1 confirmed on the
    summary surface.

FINDINGS (file as Writer work, R1-family):
F-a. build summary "N materialized" decoupled from actual writes — report
     written-count (reconcile to audit), or split "ensured N / wrote M".
F-b. run-1 audit named 2 of the "4" (settings.json, guard-bash); the other 2
     of .loom/home/. were docker-cp'd but never individually audited — audit
     completeness gap on bulk home sync.
F-c. diagnostic .loom/logs/build.log is OVERWRITTEN per run (run-1 detail
     gone) — append or rotate for forensics.
F-d. base digest drifted 0104b334→96e378d7 (ghcr base moved since 06-10);
     build correctly re-pinned. loom.lock now modified — commit to match the
     container, or plan shows lock drift forever. (Legit lock refresh, not
     debris.)

MECHANIZATION (the run-1 rubric → doctor/verify claim): "a no-change build
writes nothing AND its summary count == audit-written count". F-a is the
first thing that claim would catch.

## H-B RESOLVED + autopilot root cause (2026-06-12 ~12:10Z, Writer sidecar)
H-B (hook swap under live session benign) = PASS. The Writer "stopped" is
NOT build-induced: after run-1 materialized the new guard-bash hook (12:02),
the Writer continued operating normally — it ran the drain-hook diagnosis
(bash + reads) without error, no stranded WIP, no error handoff. The hook
swap under the live session did not break it. The stop is the SAME idle
pattern seen all day, now root-caused below.
AUTOPILOT ROOT CAUSE (Writer diagnosis): the Writer session's cwd is `/`,
not /workspace/loom (harness env: "is a git repository: false"). Claude Code
loads repo .claude/settings.json from the PROJECT dir; rooted at /, the Stop
hook (drain-inbox.sh) is NEVER REGISTERED. Even if it ran, CLAUDE_PROJECT_DIR
unset → ROOT="." = / → inbox-readable check exits 0. So the drain was never
loaded — not silently failing. This is the concrete cause behind drafts
032/033 + T27, same launch-hygiene family as the acceptEdits issue (A07) and
item 030. FIX (human/launch-wrapper): launch the Writer with cwd
/workspace/loom (or set CLAUDE_PROJECT_DIR). Until then autopilot is inert.

## EXPERIMENT STATUS: COMPLETE — H-A PASS, H-B PASS, H-C SPLIT (R1 confirmed
on the summary surface). Findings F-a..F-d + autopilot-cwd root cause filed.

---

*Promoted from `.scratch/live-build-experiment.md` 2026-06-12 (advisor, on
the standing promote-pending note in the record). Follow-up work F-a..F-d
is tracked as queue row "live-build e2e findings" + Writer inbox envelope
038. The `.scratch` copy is transit-only from this point (mail vs memory).*
