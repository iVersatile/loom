# Phase-1 close review — 2026-06-12

Contract: phase-close REVIEW GATE (human-decided 2026-06-11, draft 016;
transcription to TEAM.md pending). Reviewer = independence-ruled hat: three
fresh adversarial contexts that authored nothing in the phase (security,
architecture, harness-health), spawned by the advisor 08:05–08:15Z. Rubric:
Critical = no waiver, phase stays open · High = fix OR human accepts risk in
writing · Medium/Low → backlog, triaged. Findings name the WHAT, never the
HOW.

## Verdicts

| Dimension | Verdict |
|---|---|
| Security | **FAIL** (1 Critical, 3 High, 3 Medium, 1 Low) |
| Architecture | PASS-WITH-FINDINGS (2 High, 4 Medium, 2 Low + 1 debt note) |
| Harness-health | PASS-WITH-FINDINGS (2 High, 4 Medium, 3 Low) |

**Gate consequence: Phase-1 close is BLOCKED on C1 (Critical = no waiver)
unless the human re-scopes C1 to Phase 3 ("playbook guards", already named in
PLAN) — a re-scope is a human ruling, not an agent call.**

## Security (FAIL)

- **C1 (Critical) — guardrails declared, doctor-verified, never wired into
  built containers.** `config/playbook.yml:27-30` declares the three hooks;
  `internal/guard/guard.go:22` (used by `internal/engine/doctor.go:51`) checks
  only file presence/executability in the config source; nothing in
  build/materialize installs them — `internal/engine/materialize.go` copies
  `dotfiles:` refs only, and the materialized `config/dotfiles/claude/
  settings.json` carries a statusLine and nothing else (no permissions, no
  deny, no hooks). A built container runs its agent with zero mechanism-level
  guardrails while doctor reports them green.
- **H2 (High)** — `loom exec`/`loom shell` are an unguarded, minimally-audited
  command surface (`internal/engine/exec.go:37-49,86-119`); neither verb is in
  any deny set; guard-bash sees only the literal `loom exec -- …` string.
- **H3 (High)** — host Claude credentials bind-mounted into the container
  (`internal/engine/container.go:605-610`); the Read-tool deny on the path is
  Bash-bypassable (`cat`), and the egress deny list omits python3/node/perl//dev/tcp.
- **H4 (High)** — guard-bash is a fixed-substring deny list (trivial spacing/
  alternate-tool bypasses, `config/hooks/guard-bash:11-22`); commit-time guards
  defeated by `git -c core.hooksPath=…` which nothing denies.
- M5 — audit fail-open + tamperable (exec.go:88,107 append only when open
  succeeded; `.loom/actions.log` 0644 inside the RW mount).
- M6 — unpinned `curl | sh` installers + checksum-less Go tarball, as root
  (container.go:527-571).
- M7 — drain hook echoes untracked inbox bodies verbatim into agent
  continuation instructions (`.claude/hooks/drain-inbox.sh:138-143`); orphan
  guard is a substring test; `LOOM_SESSION_ROLE` caller-spoofable.
- L8 — settings.local.json blanket allows (UNVERIFIED scope: untracked) — see
  harness-health HIGH-1.

Checked-and-held: docker exec argv quoting; home-sentinel interpolation;
config_source root-escape rejection; `--wipe-project` typed confirmation.

## Architecture (PASS-WITH-FINDINGS)

- **F1 (High)** — teardown state surface is fiction three ways: `--clean-state`
  declared (`internal/engine/engine.go:42`, CLI-wired) but never read — silent
  no-op over "removes agent auth/memory/logs"; the `volumes` tier removes a
  `<name>-data` volume nothing creates (container.go:205); the credential-
  bearing `<name>-claude` volume loom DOES create (container.go:447,475) is
  removable by no tier or flag; the engine test passes on a mock that
  fabricates the removal.
- **F2 (High)** — plan/build convergence disagreement (the LL-012 class)
  survives on every non-tool dimension: plan grades container existence +
  tools only (plan.go:27-90); build also converges lockfile, staged dotfiles,
  agent set. FR-PLAN-003 pinned tools only.
- F3 (Medium) — exec/shell audit best-effort where SPEC-verbs#exec says
  "every exec appends an entry" (merged into backlog R1 with M5).
- F4 (Medium) — declared-but-inert flags: `build --stack/--overlay`,
  `detect --emit-playbook/--migrate` accepted, ignored (backlog R4).
- F5 (Medium) — FR↔spec verify joint is a case-insensitive substring of the
  anchor word anywhere in the cited file — nearly unfailable
  (internal/fr/verify_test.go:56-60; backlog R5).
- F6 (Medium) — `build` `--json` `result: noop` frozen in SPEC-verbs:84,
  unreachable in code (build.go:79,222-226; backlog R6).
- F7 (Low) — teardown level validation + consent exist only in the cobra
  layer; engine API is gate-free (backlog R7).
- F8 (Low) — doctor outside both conformance nets (backlog R8).
- F9 (debt) — stack knowledge hardcoded in engine switches (backlog R9).

Exit criteria: guided-run / unattended-plan-build / guardrail-block /
FR-registry / verify-green graded met or met-with-caveat (caveats = F2, F5);
evidence in docs/guided-run.md, FR-registry.yml, CI. Held: one-engine-path
exec→shell; cmd→cli→engine layering; playbook merge order; zero PLAN
citations in FRs.

## Harness-health (PASS-WITH-FINDINGS)

- **HIGH-1** — advisor `settings.local.json` carried blanket `Bash(git *)` +
  `Bash(python3 *)` allows under live `defaultMode: auto` — dissolves the
  never-auto floor (git config/ref-surgery un-prompted; python = egress past
  the curl/nc deny). S1-enabling config inside the trial window. Fix attempt
  2026-06-12 was itself denied by the auto-mode classifier (self-modification
  of permissions) → **human applies the narrowing edit**.
- **HIGH-2** — the shared tree sat on `feat/t9-shell`, so the *executing* hook
  stack was behind main (drain fyi/draft guard absent, /coordinate skill
  absent). RESOLVED same morning: tree returned to main post-push.
- MED-3 — trial ledger asserted "pre-trial" after the flip; day-1 row absent
  (fix = PR #88).
- MED-4 — stale `.scratch` session snapshots assert dead facts (backlog R10).
- MED-5 — inbox items 011/013 TAKEN by a dead session; documented handoffs,
  but nothing detects orphaned TAKEN mechanically (= backlog C2).
- MED-6 — 21 fully-merged local branches + 6 stale remote-tracking refs
  (operational sweep, this week).
- LOW-7/8/9 — flips.log field ambiguity (backlog R10); settings.local.json
  dead grants; /specmap queue row absent until its chain merges.

Held: drain core guards (role, HALT ordering, budget+reset, malformed-falls-
to-stop) incl. tests; HALT off as expected; no secrets in settings/hooks/
scratch.

## Rulings required (human)

1. **C1**: fix now vs re-scope to Phase 3 in writing. Phase-1 close blocked
   until ruled.
2. **H2/H3/H4, F1, F2, HIGH-1**: fix or written risk-acceptance each.
3. Mechanization note (gate contract): what this review hand-checked once,
   doctor inherits — candidate checks: guardrails *wired* not just present
   (C1), TAKEN-claim liveness (MED-5), snapshot freshness (MED-4).
