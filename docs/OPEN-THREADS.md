# Open design threads

Working record of in-progress design discussions. **Not an ADR** — when a thread
resolves it promotes to an ADR (RULES §3) or a spec edit, and the entry is marked
Resolved with a pointer. Kept current so an interrupted discussion resumes without
losing context.

Status: 🟡 open · 🟢 recommendation drafted · ✅ resolved (awaiting promotion).

---

## T1 — Manual-test ban for required FRs   ✅ resolved
Origin: ADR-0013 / the FR registry's `status` values.

**Decision.** A required FR's only valid coverage is an **automated** test — no
`waiver`, no `manual`. A genuinely un-automatable behavior is **not** a required FR:
reclassify it via an **automatable proxy** (test the mechanism — e.g. "bootstrap
runs to completion in a clean container," not "a human bootstraps on bare metal"),
or **downgrade** to a non-required/advisory FR with a checklist that does NOT count
toward "done".

**Human testing is NOT blocked.** It is welcomed as an out-of-band **feedback
reference** (exploratory, sanity, UX). It simply never counts as an FR's coverage
and never gates "done" — a human finding feeds back as a *new automated test*, a
*new FR*, or a *bug*, never as the FR's satisfaction. The registry `status` has no
`manual`/`waiver` value (rationale: a waiver is a trust artifact, and ADR-0005 is
mechanism-not-trust; the design test flags an agent waiving an FR to skip checks).

Promote to: the FR-registry policy section / an ADR-0013 addendum.

---

## T2 — Hermetic unit gate   ✅ resolved
Origin: LL-006 / LL-008 — three CI reds this session were unit tests that passed
locally and failed in CI because the local box lacked an env var / a binary CI had.

**Decisions.**
- **Q2.1 (A)** the local `make gate` scrubs the env by default, so **local ≡ CI**.
- **Q2.2 (A)** detection = run the unit suite in a *targeted-scrubbed* env (unset
  `LOOM_*` / `ALLOW_*`, make docker unavailable; keep the gate's own toolchain on
  `PATH`). NOT a static lint.
- **Q2.3 — PARKED** (was "yes"). Making "the unit gate is hermetic" an invariant
  FR needs a spec clause to cite (ADR-0013 `spec → FR`), but C3 forbids the AI
  authoring that RULES §5 clause. So: a **human** authors the RULES §5 hermetic
  invariant if/when they want the FR; until then no `FR-INV` for it. The mechanism
  ships regardless (below).

**Mechanism ships as plain hardening (not an FR):** Q2.1 + Q2.2 collapse into *one*
change — the gate runs unit tests in a targeted-scrubbed env, everywhere — which
directly kills the LL-006/008 class that hit 3× this session. The thing that *would*
be overkill (the AST lint, Q2.2 option b) is excluded. Impl note: "scrub" is
targeted (unset `LOOM_*`/`ALLOW_*` + hide docker), **not** `env -i` — the gate needs
go/gofmt/golangci-lint/gitleaks on PATH.

**Scope extended (2026-06-10, LL-010):** the targeted scrub also unsets the
`GIT_*` repo-redirection vars (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`,
`GIT_OBJECT_DIRECTORY`, `GIT_COMMON_DIR`) — a leaked `GIT_DIR` overrides both
cwd and `-C` (verified), so git-shelling fixtures wrote into the real shared
`.git` (incident postmortem: LL-010). Fixtures are additionally hermetic on
their own (`hermeticEnv()` + explicit `-C`), pinned by
`TestGateHermeticToGitEnv`.

---

## T3 — Phase-1 FR seeding (the bootstrap retrofit)   🟢 scope set; ADR-0013 reconciliation pending
Origin: ADR-0013 timing / bootstrap exception. Extract FRs from the frozen Phase-1
contracts + invariants + guardrail hooks, link existing passing tests, into
`docs/FR-registry.yml` (currently `requirements: []`).

**Chain (agreed, ADR-0013):** SPEC (source of truth, prose) → FR (atomic, ID'd,
machine-readable; each cites its spec section) → TEST (executable proof). `verify`
checks **both** joints — *FR ↔ test* (every FR has a passing test; every test cites
a valid FR) and *spec ↔ FR* (every FR cites an existing spec section; catches FRs
orphaned when a spec clause is removed).

**Decided.**
- **Q3.1** agent drafts the seed FRs for review.
- **Q3.3** build the `verify` check **alongside** seeding — enforced day one.
- **Q3.4** FR→test link is **registry-declared** (test names/suites in the YAML,
  resolved against a `go test -json` run), not in-code markers.

**Q3.2 — scope — CONFIRMED: broad** (include the SPEC-playbook schema/resolution
FRs; SPEC-playbook is a spec, so excluding it would blind verify's spec↔FR joint).
- *Narrow:* SPEC-verbs behavioral FRs + the global invariants + the 3 guardrail
  hooks only.
- *Broad [lean]:* also the **SPEC-playbook schema/resolution FRs** — one per
  merge/resolution rule: layer order `base→stack→overlay` (later-wins, whole file);
  lists concatenate; `rules:` explicit-by-reference (union + dedup in layer order);
  `dotfiles:` later-tier same-target-path replaces; `settings.json` base-only in
  Phase 1; format YAML-authored / JSON-on-input; lockfile granularity (per-tool
  intent/resolved/digest/source **and** base-image digest). These are frozen
  ADR-0004 / SPEC-playbook contracts that **already have tests** (playbook,
  resolver, lock packages — high coverage), so seeding them is cheap (~5–8 FRs) and
  they are exactly the "schema/resolution" category the granularity rule calls out.
  Excluding them leaves a real Phase-1 behavior gap in the registry.

**Reconciliation with ADR-0013 (conflict check — resolved).**
- **C1 — ✅ applied (PR #2 `4207fcc`).** `FR-registry.yml` `status` → `active |
  superseded` (dropped `waiver`); header records automated-only coverage.
- **C2 — ✅ applied (PR #2 `4207fcc`).** ADR-0013 now states the T1 coverage policy.
- **C3 — ⛔ DROPPED.** Governance: **the AI must not auto-author core specs**
  (RULES / SPEC-*). So the AI will not add the hermetic invariant to RULES §5.
  Consequence: **Q2.3 (hermetic gate as `FR-INV-*`) is PARKED** — it needs a
  *human-authored* RULES §5 invariant before any FR can cite it (ADR-0013's
  `spec → FR`). The hermetic-gate *mechanism* (T2 Q2.1/Q2.2) still ships as a plain
  engineering change — not a spec, not an FR.
- **C4 — agreed.** Follow ADR-0013's tiering (advisory in `make gate`; blocking at
  phase/merge/release; never per-commit).
- **C5 — resolved (split).** Dangling FR→test ref (FR cites a missing test) =
  **blocking** at the boundary (broken proof). Orphan test (no FR references it) =
  **advisory** report only (a missing FR to author, or a legit low-level test —
  never block). Triggers: test renames/deletes (dangling); new tests/behaviors
  without a registered FR (orphan).

Execution order: seed FRs against EXISTING spec clauses (verbs + invariants +
guardrails + playbook schema), each linking a passing test → build `verify` (both
joints, tiered per C4/C5). No new spec authoring by the AI.

**`verify` design (small delta — composes two patterns Loom already has).** Not a
new binary or verb. It reuses:
- *contract-check-as-a-Go-test* — like `cli.TestSpecConformance`, which parses
  `SPEC-verbs.md` and enforces it; `verify` parses `FR-registry.yml` + the spec
  files + scans `*_test.go` names.
- *integration-tier tiering* — `-tags` + a separate make target + a separate CI
  job, **absent from the per-commit gate**; this gives C4 (advisory by default via
  `make fr-verify`; blocking at the merge boundary via a CI job; never per-commit).

Delta ≈ one build-tagged test file (~150 LOC, modeled on `conformance_test.go`) +
one Makefile target (mirrors `test-integration`) + one CI job (mirrors
`integration`). Checks: dangling FR→test = blocking; missing spec section =
blocking; orphan test = advisory (C5); `covers:`/`patterns:` checked as *declared*.
Captured durably as the verify test's header docstring when built (this project's
convention — cf. `conformance_test.go`'s header).

---

## T4 — Container PATH has no single declarative owner   🟡 open
Origin: a dotfiles question — "can a project-tier `bash/path.go.sh` set PATH in the
container?" Tracing `build` revealed PATH is wired in two unrelated places, neither
playbook-declared, and they target different shell-init files.

**Observation.** PATH inside the built container comes from two split sources:
- **Hardcoded, login shell.** The provision script appends Go's PATH straight to
  `~/.profile`: `export PATH=$PATH:/usr/local/go/bin:/root/go/bin`
  (`internal/engine/container.go:312`). Stack/tool-specific, baked in engine code,
  not declared by any playbook.
- **Dotfile glob, interactive shell.** `bash/*` dotfiles materialize to
  `~/.bashrc.d/<basename>` (`internal/engine/materialize.go:47-56`) and are sourced
  by a loop appended to `~/.bashrc` (`container.go:327-329`). A new
  `bash/path.go.sh` doing `export PATH=…` *would* be picked up — but only here.

**Why it's a gap.**
- **Shell-type divergence.** `.profile` is read by login shells; `.bashrc` by
  interactive non-login; neither by non-interactive. So the two PATH sources apply
  to *different* shells — a dotfile-set PATH and the hardcoded Go PATH can disagree
  depending on how the shell is invoked.
- **Conditional wiring.** The `.bashrc.d` sourcing loop is only appended when
  `len(spec.Tools) > 0` (`container.go:127-147`); a toolless playbook copies
  `bash/*` dotfiles into `~/.bashrc.d/` but never sources them.
- **No declarative owner.** PATH is partly engine-hardcoded (Go), partly
  dotfile-expressible (anything else), with no playbook field and no single file
  that owns it. There is no `path:` field in the schema (SPEC-playbook: fields are
  `tools/rules/dotfiles/hooks/env/ports/ci`; `env:` is names-only).

**Options (no decision yet).**
1. *Status quo + document.* Accept dotfiles-for-PATH as interactive-only; note the
   `.profile` vs `.bashrc` split in SPEC-playbook so authors aren't surprised.
2. *Converge the init files.* Have the provision script source `~/.bashrc.d/*.sh`
   from `~/.profile` too (and unconditionally, not gated on `tools`), so one
   dotfile dir owns shell config across login + interactive shells; move the Go
   PATH line into a generated `bash/*` dotfile so it stops being a special case.
3. *Declarative PATH field.* Add a playbook `path:` (or `env.path:`) the engine
   renders deterministically — heavier; needs a spec change (ADR-0004/SPEC-playbook)
   and an FR. Probably overkill for Phase 1.

Lean: option 2 (engineering hardening, no spec authoring) if PATH-across-shells is
wanted; option 1 if not. Either way the divergence should be written down before a
`bash/path.*.sh` pattern is relied on.

Promote to: a small engine change + SPEC-playbook note (opt 1/2), or an
ADR-0004/SPEC-playbook edit + FR (opt 3).

---

## T5 — Lockfile doesn't pin what it claims (host-probed `resolved` + no per-tool digest)   ✅ resolved
Origin: inspecting a generated `loom.lock` before committing it; traced into
`internal/lock` + `internal/resolver` + `internal/engine`.

The lockfile is the reproducibility pin (ADR-0002) and SPEC-playbook Q3 froze its
granularity: per-tool `{intent, resolved, source, digest}` **and** a base-image
digest (`SPEC-playbook.md:14-16,125-126`). The producer doesn't meet that, in two
independent ways:

**(a) `resolved` is probed from the build HOST, not the target container.** The
resolver fills `resolved` via a host-PATH version probe (`internal/resolver/resolver.go:55`
→ `internal/engine/probe.go:18-29`; the `found` bool is discarded with `_`). A Mac
build therefore wrote host values into a lock meant to pin a `debian:bookworm-slim`
container:
```yaml
git:  resolved: git version 2.50.1 (Apple Git-155)   # the Mac's git
go:   resolved: go version go1.26.4 darwin/arm64      # the Mac's go
jq:   resolved: jq-1.7.1-apple
gitleaks/gopls/ripgrep/claude-code: resolved: ""      # not on the Mac PATH
```
`ripgrep` is `source: apt` (a container install) yet `resolved: ""` because `rg`
isn't on the host. This is a **correctness bug**: the lock records the wrong machine,
so it can't be committed as a reproducibility artifact regardless of which host
regenerates it. Fix: probe inside the resolved container (or from the install
source), not the host PATH; stop swallowing the not-found bool.

**(b) Per-tool `digest` has a field but no producer.** `LockedTool.Digest` exists
(`internal/lock/lock.go:24`, `json:"digest,omitempty"`) but the only construction
site never sets it (`internal/resolver/resolver.go:56-60`), so it silently vanishes
from YAML. Deliberately deferred in code (`resolver.go:8-9`: "pinned … (later);
Phase 1 records the resolved version"). **But SPEC-playbook overstates reality** —
it says digests are frozen *and* "code implements them … the example already
reflects this shape" (`SPEC-playbook.md:5,18`). Spec↔code drift: either implement
per-tool digests or amend the spec to mark them Phase-2.

Base-image digest is the one part that **works** (`internal/engine/build.go:85-89`,
`container.go:87-96` via `docker buildx imagetools inspect`; `build_test.go:130`).

**Options (no decision yet).**
1. Fix (a) now (container/source probe) — it's a plain correctness bug, no spec
   change; gate a lock-commit on it.
2. Resolve (b) by spec edit: mark per-tool digest Phase-2 in SPEC-playbook so the
   spec stops claiming it's implemented (human-authored, not AI — RULES §5 / C3).
3. Or implement per-tool digest (heavier; pin against the pulled image as the
   resolver comment envisions).

Lean: (1) is a must before any `loom.lock` is committed; (2) is the honest
short-term reconciliation for the digest half. Until both, **do not commit a
generated `loom.lock`.**

**Resolution (2026-06-10, `fix/t5-lock-fidelity`):** options (1)+(2), as leaned.
(a) Build now resolves `resolved` by probing **inside the converged container**
(`ContainerRuntime.Probe` via a login shell; `lock` re-pinned post-provision,
carried forward pre-container so unchanged setups stay no-ops); not-found stays
`""` — never a host value. Covered by **FR-BUILD-007** /
`engine.TestBuildLockRecordsContainerVersions`. (b) SPEC-playbook's lockfile-
granularity decision gained an honest *Phase status*: per-tool `digest` producer
is **Phase 2** (field in schema, not yet populated); base-image digest produced.
A regenerated `loom.lock` is now committable; per-tool digest (option 3) remains
the Phase-2 follow-up.

---

## T6 — `build --dry-run` mutates (violates plan-semantics contract)   ✅ resolved
Origin: a Mac `./bin/loom build --dry-run` that was expected to preview but actually
provisioned.

`--dry-run` is documented as "preview changes without applying (plan semantics)"
(top-level flag help) and `plan` is the never-mutates verb (`SPEC-verbs.md`). But a
`build --dry-run` run did real work:
- reported `created (container loom-loom-dev, lock_written=true, 3 materialized)`;
- actually provisioned — `.loom/logs/build.log` shows real `go: downloading …` and
  `+ grep -q bashrc.d /root/.bashrc` (container commands executed);
- rewrote `loom.lock` (`resolved_at` advanced) and wrote staging files under
  `.loom/home/…`.

A dry-run must do none of these. Either `build` ignores the `--dry-run` flag, or it
threads it but the container/materialize/lock-write path doesn't honor it. Net
effects: false "preview" that mutates state, and (combined with T5) it wrote a bad
lock unprompted.

Next step: trace where `--dry-run` is read in the `build` path (cmd/loom + the build
engine) and confirm whether the flag reaches the mutating steps at all. Likely a
guard missing before container create / materialize / lock-write.

**Correction to an earlier audit note:** `--dry-run` WAS specced — SPEC-verbs
*Global conventions* ("`--dry-run` where an action would change state (alias of
`plan` semantics)") and `update` ("`--dry-run` == `plan`"). The thread title was
right all along: a contract violation, not unspecced growth. (The earlier "spec
the flag" direction was based on the wrong unspecced reading and is superseded.)

**Root cause (confirmed in PR #8):** the flag was registered as a persistent flag
in `internal/cli/root.go` but **no verb ever read it** — `build` ran its full
mutating path unconditionally. A promise with no mechanism.

**Resolution (user decision, 2026-06-09): abandon `--dry-run`; `plan` is the one
preview path.** Implemented in **PR #8** (`fix/t6-remove-dry-run`): the flag is
removed from the CLI, and SPEC-verbs is amended (audited `ALLOW_SPEC_CHANGE` on
explicit instruction; merge = human acceptance) — the global convention now states
"No `--dry-run`" with the T6 rationale, and `update` points at `plan`. One preview
surface keeps the read-only promise enforceable; it stays covered by FR-PLAN-001/002
(no FR cited the removed clause; `fr-verify` green).

---

## T7 — `build` converge skips container `$HOME` re-sync on dotfile-only change   ✅ resolved
**Resolution (2026-06-10, `fix/t7-home-resync`):** option (1) — a **home
sentinel** (`/var/lib/loom/home`), the ADR-0011 pattern applied to the $HOME
surface ADR-0015 materializes. `homeDigest()` fingerprints the staging tree
(rel path + mode + content); `needsHomeSync()` mirrors `needsReprovision`;
`Ensure` converges on either sentinel going stale. Home drift triggers only
the `docker cp` + sentinel write — provision stays gated on the toolset digest
(a dotfile change never re-runs apt/go-install; the T4 interplay note held).
The misleading-status defect (option 3) dissolves by construction: "exists"
now means nothing needed syncing. Tests: `TestHomeDigestDetectsDotfileChange`,
`TestNeedsHomeSync` (unit), `TestE2EDotfileChangeConverges` (integration:
edit → rebuild → content in container + status converged → third build
"exists"); linked from FR-BUILD-004. One-time effect on merge: existing
containers have no home sentinel, so their next build re-syncs $HOME once.
Entry kept below for the original analysis.

Origin: a real change (richer `claude/statusline.sh`) was committed, built, and
reported `converged … 3 materialized` — but a session **inside** `loom-loom-dev`
still saw the old statusline. `docker exec … cat /root/.claude/statusline.sh`
confirmed the container kept the old file while host staging had the new one.

**Root cause (confirmed in code).** The container `$HOME` re-sync is gated on the
**toolset digest only**, so a dotfile-only change never triggers it:
- `toolsetDigest` hashes only `tools` (`Name|Source`) — nothing about dotfiles
  (`internal/engine/container.go:228-239`).
- In `Ensure`, an existing container with an unchanged toolset **early-returns
  `"exists"` and skips the `docker cp` of `$HOME`** (`container.go:119-120`). The
  reconcile copy (`container.go:122-126`) sits *after* that return, so it runs only
  when tools changed or a prior provision was interrupted (`needsReprovision`).
- Result: changed dotfiles reach host staging `.loom/home` but never an
  already-built container. Only `--force`/teardown (the create path,
  `container.go:138-141`) copies `$HOME`.

**Misleading status message (second defect).** `build` prints `converged … N
materialized` driven by the **host** `changed` flag (`materializeDotfiles` wrote
staging, `build.go:113-125,172-173`), even though `Ensure` returned `"exists"` and
pushed nothing to the container. `build.go`'s status switch has no `"exists"` case
(`build.go:146-166`), so the message reports staging state, not container state — a
user reasonably reads "3 materialized" as "3 files are now in my container."

**Why it matters.** This is the most user-visible of the current bug cluster: the
declared-desired-state promise (edit config → `build` → container reflects it) is
silently broken for the entire dotfiles surface (statusline, prompt, claude
settings, any future `bash/*`). The only way to apply a dotfile edit today is a
destructive full rebuild.

**Options (no decision yet).**
1. Extend the reconcile trigger: fold a **dotfiles/home digest** into the sentinel
   (or compare staging vs container) so `needsReprovision` (rename → `needsConverge`)
   also fires on `$HOME` drift; always `docker cp` staging when it differs. Cheap,
   non-destructive, matches reconcile intent.
2. Always `docker cp` staging on every build (idempotent; cheap for a few files) and
   keep the provision (tool install) gated on the toolset digest as today.
3. At minimum, fix the **message**: add an `"exists"` status case so `build` does
   not claim `converged/N materialized` when nothing reached the container.

Lean: (1) or (2) for the real fix + (3) regardless (honest status). Note the
`len(spec.Tools) > 0` gate on the `.bashrc.d` sourcing loop (T4) interacts here:
the home-sync fix should not depend on tools being present.

Promote to: an engine bugfix (home-drift reconcile) + a status-accuracy fix, with a
regression test (dotfile edit on an existing container → file present in container,
status reflects it); FR once `verify` covers the build reconcile path.

---

## T8 — `agents:` are declared but never installed   ✅ resolved
Origin: the container loom builds (`loom-loom-dev`) has the materialized `.claude/`
config but **no `claude` binary** — investigating why the statusline/agent config
was inert revealed agents are never provisioned.

**Root cause (confirmed).** `ContainerSpec.Tools` is fed by `toolInstalls()`, which
copies only `resolution.Tools` — **agents are excluded** (`internal/engine/build.go:178-184`).
Agents are *only detected* (presence-probed, `internal/engine/detect.go:68-70`),
never installed. So `agents: [claude-code]` (base playbook) produces nothing; the
lock records `claude-code.resolved: ""`. The container has agent config without the
agent program.

**Why it matters.** This is the capability gap that makes `loom-loom-dev` an
uninhabitable dev env: the whole AI-first premise (ADR-0005) is that the container
hosts the agent, but loom installs none. Blocks "actually use the loom container."

**Scope also needs credentials.** A `claude` binary still needs auth to run. The
`devenv` sandbox gets this by bind-mounting the Mac's `~/.claude` (creds + settings).
loom must either bind-mount `~/.claude` creds or inject a token via the secret store
— **no baked secrets** (RULES). Installing the binary without solving creds is half
a fix.

**Options.**
1. Provision agents like tools: add an agent install path (per-agent source — npm/
   curl installer for claude-code, etc.), pin in the lock (`resolved`/`digest`),
   gate reinstall on an agent-set digest (cf. toolset digest).
2. Bind-mount the host agent install + `~/.claude` into the container instead of
   installing (lighter; mirrors how `devenv` works today).
3. Hybrid: install the binary (1), mount only credentials (2).

Lean: (3) — own the binary so the container is self-contained, mount only secrets.
PLAN link: realizes `docs/PLAN.md` → *Open items → "Working env for building Loom …
Claude Code in-container (dogfood)"* (its fallback "mount loom + claude into
/usr/local/bin in-container" is the agent half of that item).
Promote to: an engine capability + a creds decision (possibly an ADR — interacts
with ADR-0005 and the secret-store design); FR once `verify` covers agent install.

**Resolution (2026-06-09/10, PR #7 merged):** option (3) as leaned — `build`
installs declared agents (claude-code native installer, `~/.local/bin` on PATH);
the provision sentinel digest folds in the agent set. Credentials decided in
**ADR-0014 (Accepted)**: interactive in-container OAuth login primary; env-token
demoted to CI-only; host creds-file mount a Linux-only no-op-on-mac secondary.
Covered by **FR-BUILD-006**. Durability of the login → T14 (resolved); its
human-only nature → T15 (open).

---

## T9 — no verb to enter the container (`shell`/`enter`)   🟢 decided — awaiting the human clause PR
**Decision record (2026-06-10, discussion with the human; clause text is
human-authored per C3 — this entry records the rulings, it is not the spec).**
Two verbs; **`loom exec -- <cmd>` ships first**, `loom shell` is sugar over the
same engine path (TTY + `bash -l` as the command) and may trail by a PR. The
eight rulings, binding for ADR-0016 / implementation / FRs:
1. Two verbs: `exec` (one-shot) + `shell` (interactive sugar).
2. `exec` requires a command — bare `loom exec --` is an immediate usage error
   (exit ≠ 0), never an interactive fallback (AI-first: a hang is worse than
   an error). The command's exit code propagates verbatim.
3. Working directory: the project mount `/workspace/<project>` — devcontainer
   `exec` semantics are the reference (ADR-0003 neighborly).
4. Login environment: command runs with login-shell env so the provisioned
   PATH applies (the `Probe` `sh -lc` lesson).
5. **No `--json` on either verb** — `exec` is transparent passthrough; the
   structured surface is the **audit entry** (command, exit code, action id
   per exec; `shell` logs session-open only, no command capture — human
   privacy/noise call). The clause states the RULES §5 exemption;
   `cli.TestSpecConformance` may need teaching in the impl PR (test code is
   agent territory).
6. Lifecycle: stopped container → `docker start` then enter (idempotent
   bring-up); absent → error with hint ("no container — run loom build"),
   non-zero exit. The verb never creates or provisions.
7. User: the configured container user (root today; parameterize-ready, T10).
8. **No command filtering**: the verbs are doors, not checkpoints — no
   authority beyond and no filtering before the container's guard envelope
   (T16's territory). No verb-level blocklist, ever.
Remaining sequence: human clause PR (the only blocker) → ADR-0016 acceptance →
impl PR (exec: cli + engine via the runtime interface, unit + e2e
`loom exec -- make gate`, FR registration) → shell PR. FR drafts are staged in
ADR-0016, NOT the registry (fr-verify needs the clause anchor to exist first).
Original thread kept below for the pre-decision record.

Origin: after `build`, there is no loom-native way to get *into* `loom-loom-dev`;
the user fell back to a separate sandbox.

**Observation.** Verbs are `build / plan / detect / teardown / doctor`
(`cmd/loom/main.go`) — none open a shell in the container. The container is built
but has no door, so the materialized env (dotfiles, tools, future agent) is
unreachable through loom. Today the only way in is raw `docker exec -it
loom-loom-dev …`.

**Why it matters.** "Container as dev env" requires an ergonomic entry point. Raw
`docker exec` works as a stopgap (so this does *not* block first use), but a verb is
what makes the loop loom-native and lets loom control the entry user/shell/env (ties
to T10 non-root and T8 agent).

**Note: this is a new verb = a contract change.** Per RULES §2 / §5 (C3), a new
SPEC-verbs entry must be **human-authored** (the AI must not author core specs). So
this thread captures the need + shape; the spec clause + ADR are a human step.

**Options.** `loom shell` (interactive `docker exec -it <user> bash -l`) vs `loom
enter` vs `loom exec -- <cmd>` (one-shot). Lean: `loom shell` for interactive +
`loom exec --` for scripted, both honoring the non-root user (T10) and `--json` where
sensible (RULES §5 human+json — though an interactive shell is exempt).
Promote to: a SPEC-verbs addition (human-authored) + ADR + engine impl + FR.

---

## T10 — container runs as root; should be a non-root `dev` user   🟡 open
Origin: `loom-loom-dev` is `root@/root` (confirmed `docker exec … whoami` → root),
which is why dotfiles materialize to `/root` and the home-target confusion arose.

**Observation.** The base image (`debian:bookworm-slim`) defaults to root; loom does
no user setup and hardcodes `/root` as the home (`container.go` cp to `:/root/`,
provision writes `/root/.bashrc`/`.profile`). Running an interactive dev container as
root is a hygiene/security smell and diverges from the `devenv` sandbox (non-root
`dev`, `HOME=/home/dev`) the user is accustomed to.

**Why it matters.** A non-root `dev` user (uid 1000, `HOME=/home/dev`) would: match
conventional dev containers, reduce blast radius, align with `devenv`, and make the
home-target explicit instead of an implicit `/root` assumption. It interacts with
T8 (agent + creds land in the user's home) and the materialize path (must target the
user's `$HOME`, parameterized, not hardcoded `/root`).

**Options.**
1. Create a `dev` user in provision; set `HOME=/home/dev`; parameterize materialize/
   provision on the resolved home; `docker exec` as `dev`.
2. Keep root for Phase 1 (simplest) and revisit — but this perpetuates the smell and
   the `/root` hardcode.

Lean: (1), bundled with T8/T9 so the inhabit-the-container work lands coherently.
Promote to: an engine change (user + parameterized home) + an ADR note (container
user policy); FR once covered.

---

## T11 — container name `loom-loom-dev` is awkward (doubled "loom")   ✅ resolved
Origin: the dogfood container is named `loom-loom-dev`; the doubled "loom" reads
badly and the user wants `loom-dev`.

**Cause.** `containerName(project) = "loom-" + project + "-dev"`
(`internal/engine/container.go:261-262`). With the loom project's `name: loom`, the
`loom-` prefix collides with the project name → `loom-loom-dev`. For other projects
it's fine (`loom-prompiler-dev`); the doubling is specific to loom-on-loom.

**Recommendation.** Drop the name-prefix as the namespacing mechanism; name the
container `<project>-dev` (→ **`loom-dev`**, `prompiler-dev`) and move the
"loom-managed" marker to a **docker label** (e.g. `loom.project=<name>`,
`loom.managed=true`). Benefits: no doubling, still discoverable
(`docker ps --filter label=loom.managed`), and decouples identity from display name.
Migration: a rename is a new container identity — `teardown` the old + `build` the
new, or a one-time `docker rename`; note the action log + any `detect` that keys on
the name.

Options: (a) `<project>-dev` + labels [lean]; (b) keep prefix but de-dupe when
`project == "loom"` (hacky, special-case); (c) `loom/<project>` (slashes are
awkward in container names). 

**Decision (user, 2026-06-09): option (a)** — container name is `<project>-dev`,
loom-managed marker moves to docker labels. **Requires an audited SPEC-verbs edit:**
the `build --json` example hardcodes `"name":"loom-loom-dev"` (`SPEC-verbs.md#build`),
so the rename touches a frozen contract — human-authored or `ALLOW_SPEC_CHANGE=1` on
explicit instruction, alongside the ADR-0001 naming note. Scheduled in the P0 engine
batch with T13+T14 (all create-time changes: one rebuild, one re-login).

Promote to: an engine change (name template + labels) + SPEC-verbs example edit +
ADR-0001 naming note; FR once covered.

**Resolution (2026-06-10, PR #10 merged):** option (a) — `containerName` is
`<project>-dev` (→ `loom-dev`); the managed marker is the labels
`loom.managed=true` / `loom.project=<name>`. SPEC-verbs examples updated and the
naming convention recorded in the ADR-0001 addendum. Covered by
`engine.TestContainerName` / `engine.TestCreateRunArgs`.

---

## T12 — retire `devenv`; make `loom-dev` the single dev container   🟢 decision drafted
Origin: clarifying the three-environment confusion (Mac host / `devenv` / the loom
container). **Decision direction (user):** archive `devenv`, later remove it.

**Facts established this session.**
- This interactive/agent session runs in a container named **`devenv`** (not the
  loom container): `/.dockerenv` present, `linuxkit` kernel, `dev@/home/dev`,
  Debian 12. It **bind-mounts the Mac's** `~/.claude`, `~/.gemini`, `~/.codex`,
  `~/.gitconfig`(ro), and `/Users`→`/workspace` (Docker Desktop file share).
- `devenv` has **no docker** (no binary, no socket) and **has go** + the agent
  harnesses + linuxbrew tools.
- `loom-loom-dev` (the loom container) is `root@/root`, has tools but **no agent**
  (T8) and **no entry verb** (T9).

**Why `devenv` is redundant (agreed).** My earlier "devenv = outer bootstrap" framing
was **wrong**: `loom build` needs docker, which `devenv` lacks, so the build does
*not* run here — it runs on the **Mac host** (docker + the cross-compiled
`loom-darwin-arm64`). So `devenv` is neither the build host nor (post-amendment) the
dev env. Its two real roles — *run the agent* and *provide the go toolchain* — both
move to `loom-dev`: go is already a loom-dev tool (`go@1.26`), and the agent arrives
with T8.

**"Usable `loom-dev`" criteria (the bar for cutover).**
1. T8 — `loom-dev` installs the agent + has working credentials (env token primary;
   see ADR-0014 / the session verification finding).
2. T9 — an entry path exists (verb, or documented `docker exec -it`).
3. **T13 — the project repo is mounted into `loom-dev`** (today it is not; you
   cannot work on code there without it).
4. Harness home — the parts of `~/.claude` you depend on (hooks/guards, memory,
   skills, permissions) are provided, not just `settings.json` + statusline.
5. The loom **dogfood loop runs in `loom-dev`**: `make gate` (unit tier; go present)
   works; integration tier stays on **CI** (already true) or `loom-dev` gains docker
   access (DinD/socket — likely FC-001) if local integration is wanted.
   **✅ verified 2026-06-10, by the loop itself:** `make tools && make gate` ran
   green inside `loom-dev` (`gate: PASS` — fmt-check, vet, lint, spec-check,
   unit tests, gitleaks). Caveat that keeps this criterion honest: lint ran
   only after `make tools` go-installed golangci-lint, an undeclared gate
   dependency repaired in-container — the declaration gap and its mechanism
   fixes are T19. Criterion 5 holds once that fix merges and a rebuild
   provisions the gate toolchain from the playbook alone.

**Counter-argument considered & rejected:** "keep `devenv` to compile the loom
engine (Mac has no go)." Rejected because `loom-dev` already declares `go@1.26`, so
it can build the engine and run the unit gate itself; integration is a CI concern.
Once 1–5 hold, `devenv` adds nothing.

**Operating model — NO side-by-side use (user decision).** `devenv` and `loom-dev`
must **not** be used concurrently for real work: they share one credential and one
subscription, with the contention/auth-rotation/quota risks catalogued in
`.scratch/session-start-verification.md`. The plan is a clean **cutover**, not
parallel operation. (Brief overlap is allowed *only* for the one-time verification
pass — exec-in checks, then stop.)

**Cutover + deletion timeline.**
1. Reach criteria 1–5 → declare `loom-dev` usable. *(Not yet — T8 done; T13 +
   harness home + verified auth pending.)*
2. On that day: **archive `devenv`** — stop using it, move real work to `loom-dev`.
   Keep it stopped (not deleted) as a rollback safety net.
3. **Delete `devenv` ~2 weeks after archival** (archive-date + 14d), assuming no
   rollback was needed. The 2-week clock starts at *archival*, not now — so no fixed
   calendar date yet; set it when criteria 1–5 are met. Worth a `/schedule` reminder
   at that point.

PLAN link: realizes the `docs/PLAN.md` Phase-1 **goal** *"Dogfood: build Loom inside
the container Loom builds"* and its *Open items → "Working env for building Loom …
(dogfood)"*. NB: dogfood is a Phase-1 **goal + non-blocking open item**, NOT an
enumerated exit criterion — so this gap does not gate Phase-1 close as PLAN is
currently written (promoting it to an exit criterion is a human-authored PLAN edit).
Promote to: an ADR recording the single-dev-container model + `devenv` retirement
(human-authored, since it touches the env/topology decision), once criteria met.

---

## T13 — `loom-dev` has no project/repo mount   ✅ resolved
Origin: session verification — a `claude` session in `loom-dev` had no code to work
on. `docker inspect loom-loom-dev --format '{{json .Mounts}}'` returned `[]`, and
`ls /workspace` → No such file. **Confirmed:** loom never mounts the project.

**Root cause.** `Ensure`'s `docker run` is bare — `run -d --name <name> <image>
sleep infinity` (`internal/engine/container.go`), with no `-v` for the project. The
materialized `$HOME` is `docker cp`'d in, but the working tree is not. The
`/workspace` seen in `devenv` is **devenv's** Docker-Desktop file share, not loom's
and not shared with `loom-dev`.

**Why it matters.** ADR-0001 is *container-per-project*, yet the project isn't in
the container — so `loom-dev` can't host real dev work. This is a hard blocker for
T12's "usable `loom-dev`" bar, alongside T8 (agent ✓) and the harness-home gap.

**Recommendation.** Bind-mount the project root (the dir holding `loom.yml`) into
the container at a fixed path, RW so edits sync host↔container (the devcontainer
model, ADR-0003). Parameterize on the container user's `$HOME` (interacts with T10
non-root) rather than hardcoding. Add to `ContainerSpec` (e.g. `ProjectMount{Host,
Container}`) and to `docker run` as `-v host:container`. Set at create only
(docker can't add `-v` live → `--force`, same constraint as T8 creds/env).

**Options.**
1. *Bind-mount host repo RW* [lean] — live edits both ways; matches devcontainer.
2. *Copy/clone repo into the container* — more isolated, but edits don't reach the
   host and drift from git; rejected for a dogfood loop.
3. *Named volume* — persists across rebuilds but detaches from the host tree; wrong
   for editing source you also touch from the host.

**Caution (ties to T12 no-side-by-side).** With the repo bind-mounted **and**
`devenv` live, two sessions share one working tree → `index.lock`/checkout races,
clobbering edits (concurrency risk #8). The T12 cutover (no side-by-side) is what
keeps this safe; the mount should land with that operating model, not parallel use.

PLAN link: this is the `docs/PLAN.md` *Open items → "Working env for building Loom"*
**fallback made concrete** — "mount loom … in-container" is exactly the project-mount
this thread specifies (the repo half of the dogfood working env).
Promote to: an engine change (project mount in `ContainerSpec` + `docker run`),
parameterized for T10; a note in ADR-0001/0003 (mount model); FR once covered.

**Resolution (2026-06-10, PR #10 merged):** option (1) — the project root
(`loom.yml`'s directory, absolute) bind-mounts **RW** at `/workspace/<project>`
at create (`ContainerSpec.ProjectDir`). Mount model recorded in the ADR-0001
addendum. Create-time-only as designed: changing it requires `--force`. Covered
by `engine.TestCreateRunArgs`. The two-writer caution stands until T12's cutover.

---

## T14 — agent credentials are lost on every rebuild   ✅ resolved
Origin: `loom-dev` was made usable (T8) by completing an **interactive OAuth login
inside the container** — the only path that works for the interactive TUI after the
file-mount and env-token routes failed (see below). Confirmed working: `claude`
starts authenticated in `loom-loom-dev`.

**The gap.** That login writes `/root/.claude/.credentials.json` in the container's
**writable** home, so it persists for the container's *life* — but `build --force` /
`teardown` (`docker rm`) destroys the container and the file with it. A plain
*converge* build does NOT (the container survives), so the loss trigger is
specifically a forced rebuild / teardown. Net: **re-login required after every
`--force`.** Acceptable as a stopgap; not acceptable long-term.

**Why the simpler paths don't solve it (verification findings, this session).**
- *Host creds file mount/copy* — DEAD on a **macOS** host: Claude Code stores creds
  in the **Keychain**, so there is no `~/.claude/.credentials.json` to mount or
  `docker cp` (`ls` on the Mac → No such file; `docker inspect … .Mounts` → `[]`).
  Would work only on a Linux host that has a real creds file.
- *`CLAUDE_CODE_OAUTH_TOKEN` env passthrough* — works for **headless** `claude -p`
  only, **not** the interactive TUI; and `docker run -e VAR` stores the value in the
  container's `Config.Env`, leaking it into `docker inspect` + shell history. Dropped
  from `loom.yml` default; kept as an optional CI-only path (ADR-0014).

**Options (no decision yet).**
1. **Persist a creds volume** — bind a small named volume at `~/.claude` (or just the
   creds file) so it survives `docker rm`; re-login only when the token actually
   expires. Simplest durable fix.
2. **Re-seed creds on build** — have loom copy a stored creds file back into the
   container home on (re)build, sourced from a host location or secret store.
   Depends on a host creds file existing (false on macOS) → weak unless paired with
   a loom-managed creds store.
3. **`apiKeyHelper`** — a script in `settings.json` that fetches a key per request
   from a secret manager; most secure, most setup.

Lean: (1) for the dogfood loop now (a creds volume excluded from the reset tier),
revisit (3) for a real secret-store integration. Interacts with the **harness-home**
thread (memory/hooks/skills also need to survive rebuild) and **T13** (mounts).

PLAN link: a specific case of `docs/PLAN.md` *Open items → "Agent-initiated container
lifecycle with task continuity"* — that item observed (2026-06-09) a `docker stop
devenv-dev` losing a live session mid-task; creds are one piece of the container
state that must **persist outside and rehydrate on bring-up** (memory/session
continuity are the rest, in the harness-home thread).
Promote to: an engine change (persist `~/.claude` across rebuild) + an ADR-0014
addendum; FR once covered.

**Resolution (2026-06-10, PR #10 merged):** option (1) — a named volume
`<container>-claude` mounts at `~/.claude` when an agent is declared, so the
in-container login survives `--force`/`teardown`; re-login only on real token
expiry. Deliberately excluded from the `volumes`/`reset` teardown tiers (agent-
auth wipe is the opt-in `--clean-state` tier). Recorded in the ADR-0014
addendum; covered by `engine.TestCreateRunArgs`. Option (3) `apiKeyHelper`
remains the T15 path; the mutable-state half of harness-home now rides this
volume (see T16).

---

## T15 — the working auth path is human-only; AI-first auth needed   🟡 open
Origin: ADR-0014 landed on **interactive in-container OAuth login** as the only
path that authenticates the interactive TUI — but completing that flow requires a
human with a browser. An autonomous agent cannot perform it.

**Observation.** Of the three mechanisms tested this session (ADR-0014): the host
creds-file mount is dead on macOS (Keychain, no file); the env token authenticates
headless `claude -p` only and leaks into `Config.Env`/`docker inspect`/shell
history; the browser OAuth works but is human-gated. Net: the loom container
becomes inhabitable only after a human ritual — repeated after every `--force`/
`teardown` until T14 lands.

**Why it matters.** ADR-0005 makes the AI agent a **first-class user**: the
environment must be operable by an autonomous agent end-to-end. If the container
loom builds can only be authenticated by a human, the AI-first premise breaks at
step one — an agent cannot bring up (or recover, per the PLAN task-continuity
item) its own working env. Non-interactive, **leak-free** credential acquisition
is a first-class capability gap, not an ergonomic nit.

**Options.**
1. **Secret store + `apiKeyHelper`** — `settings.json` helper fetches a key per
   request from a secret manager; non-interactive, no secret at rest in any loom
   artifact or Docker metadata. The real AI-first shape (ADR-0014's "revisit if").
2. **Human-minted long-lived token in a secret store** — `claude setup-token`
   once, loom injects at exec-time (not `docker run -e`, avoiding the Config.Env
   leak). Still headless-only for the TUI, and a year-long token is a wide blast
   radius (ADR-0014 rejected it as default).
3. **Creds volume (T14 option 1)** — one human login made durable across rebuilds;
   reduces the ritual to token-expiry frequency but does not eliminate the human.

Lean: (3) now, bundled with T14, to cut ritual frequency for the dogfood loop;
(1) as the actual answer — the agent authenticates itself through mechanism, with
no secret value an agent could exfiltrate from loom's artifacts (ADR-0005
mechanism-not-trust design test).
Promote to: an ADR-0014 addendum or successor ADR (credential-acquisition policy,
interacts with the secret-store design); FR once a non-interactive auth path is
covered by `verify`.

---

## T16 — harness home: loom provides `settings.json` + statusline, not the rest   ✅ resolved (ADR-0015 Accepted)
**Status (2026-06-10):** promoted to **ADR-0015**, now **Accepted** (human
acceptance via PR #24 merge —
`docs/decisions/0015-harness-home-config-vs-state.md`). Remaining work is
implementation, tracked in the PLAN tactical queue ("T16 engine work"; T7
precondition fixed on `fix/t7-home-resync`). The ADR resolves this thread's
open questions:
config/state split at the volume seam, `harness:` section
(explicit-by-reference) over generalized `dotfiles:`, two-tier policy split
confirmed per T18, memory seeds empty, session continuity as declared hooks.
T7 is recorded as a precondition for the engine work. Entry kept below for the
full lean and the verification record.

Origin: the loom-dev verification pass (`.scratch/session-start-verification.md`)
— the predicted-LOSE list confirmed. The materialized `~/.claude` is statusline-
only; everything else the dev experience depends on is absent. This is T12's
usability criterion 4, the last unaddressed dogfood blocker besides cutover
itself.

**What's missing in `loom-dev` (confirmed absent).**
- **Hooks/guards:** SessionStart continuity snapshot, guard-bash, branch-guard,
  session-end — none run; `settings.json` is statusLine-only.
- **Memory:** `MEMORY.md` + auto-memories (`~/.claude/projects/<proj>/memory/`).
- **Skills / agents / plugins:** not materialized.
- **Permissions allow/deny:** no allowlist, so every session re-prompts.
- **Git identity:** `~/.gitconfig` neither mounted nor set (small, same family).

**The shape of the fix — split by mutability (lean).**
1. **Declarative config** (hooks, skills, permissions, settings, gitconfig
   identity) is *playbook-declared and materialized* like dotfiles, from the
   config source — versioned, reviewable, re-converged on `build` (ADR-0002/
   0006: declared, not hand-made). Note the engine already plans to bake
   guardrail hooks into built envs (protect-paths header, "Work 6") — the
   harness hooks ride the same mechanism.
2. **Mutable state** (memory, session history, creds) lives in the **T14 agent-
   home volume** — survives rebuild, never bind-shared with the host or another
   container (the session-journal corruption risk from the verification notes).

The boundary cuts cleanly at `~/.claude`: config materializes INTO the volume on
each build (docker cp through the mount), state accretes in it. Alternatives
considered in the verification notes: bind-mounting the host `~/.claude`
(rejected — shadows materialized config, shares mutable state across writers,
dead on macOS for creds); copying once at create (rejected — config drifts from
the source, exactly T7's class of bug).

**Open questions.**
- Playbook schema: does `dotfiles:` generalize (it already targets `$HOME`
  paths), or does a `harness:` section earn its keep (hooks/skills/permissions
  have semantics dotfiles don't — e.g. executable bits, per-project memory dirs)?
- Permissions/guard policy is *policy*: does its source belong in `rules:`
  (explicit-by-reference) rather than dotfiles? Interacts with the two-tier
  config (ADR-0004) — base-wide guards vs per-project allowlists.
- Memory seeding: start empty, or import the host's project memory once at
  volume creation (continuity vs a clean cut)?
- T10: everything here must target the parameterized `$HOME`, not `/root`.

**Session continuity is part of this thread (added 2026-06-10, at cutover).**
Session restarts are a certainty (model switch, rebuild, crash), so "how does a
fresh agent session regain project context" needs a reusable convention, not a
hand-written prompt each time. Three layers, by where each belongs:
1. *Now (repo convention):* a tracked-tree handoff doc the agent reads at
   session start (`.scratch/loom-dev-session-start.md` is the first instance) —
   works because the mounted repo is the one surface every session sees. The
   broader principle already holds: OPEN-THREADS + docs are the durable memory;
   chat is not.
2. *Playbook (this thread's scope):* the SessionStart continuity hook — the
   mechanism that produced devenv's session snapshots — is exactly the kind of
   harness config item (1) above materializes from the config source. ADR-0015
   should treat "agent regains context on session start" as a first-class
   harness-home requirement, alongside guard hooks: snapshot-on-end +
   orient-on-start, declared in the playbook, not hand-carried.
3. *Spec (later, human-authored):* the durable contract is PLAN's open item
   "agent-initiated container lifecycle with task continuity" — a RULES/SPEC
   clause + FR once the mechanism exists; not before (ADR-0013 spec→FR order).

Promote to: an ADR (harness-home strategy — config materialized vs state in
volume, INCLUDING session continuity hooks), then engine work (materialize
hooks/skills/permissions + executable bits), then FRs per behavior. Blocks T12
criterion 4; design together with the ADR-0014 addendum (T14) it builds on.

---

## T17 — activity scripts: operational history as a verb incubator   🟢 convention adopted
Origin: the loom-dev migration (T11/T13/T14 cutover) needed a recorded, re-runnable
procedure instead of a terminal scrollback; the same was true of earlier manual
sequences (the leaked-token search-and-clear after the env-token experiment, the
session-start verification checklist in `.scratch/`).

**Problem.** Operational activities (migrations, sweeps, verifications) happen as
ad-hoc command sequences. They leave no durable record, are not re-runnable, and —
most importantly — their recurrence is the strongest signal that the **engine has a
capability gap**, a signal currently lost.

**Convention (adopted, `scripts/README.md`).** Activity scripts are tracked in
`scripts/`: POSIX sh, header block (Purpose / Origin / Reuse / Runs-on),
confirmation-gated destructive steps, no secrets, mutations through `loom` verbs
where one exists. Each script declares its lifecycle stage:
`one-off → recurring → verb-candidate`. Promotion to the engine follows the loom
way — spec clause (human-authored, C3) → ADR if needed → implementation → FR —
and the script is then **deleted with a pointer**, never left to drift beside the
verb it became.

**The signal worked immediately — mappings already visible.**
- *Leaked-token search & clear* → already specced, unbuilt: `detect`'s credential
  scan ("present keys/credentials across known locations … detect + report by
  default") and `teardown --clean-state` (agent auth) are its homes. The script
  stage can cover the gap until those land; its existence is the implementation
  prompt.
- *Migration on container-identity change* (`migrate-loom-dev.sh`) → recurs
  whenever naming/mounts change at create-time; candidate engine behavior: build
  detects an old-identity container (by `loom.project` label, name-independent
  post-T11) and offers/performs the replacement.
- *Session-start verification checklist* → `doctor` checks (agent on PATH, repo
  mounted, volume present, auth alive).

**Boundary.** `scripts/` is for *operator activities*. It is NOT a side door for
engine logic: anything idempotent-and-recurring that mutates project/container
state belongs in a verb (RULES §5 — auditable, idempotent, --json), and the
lifecycle above exists to force that conversation rather than accrete a shadow
CLI in shell.

Promote to: per-script promotions as above (each its own spec/ADR/FR step); the
convention itself stays a working convention in `scripts/README.md` unless it
earns a RULES §-clause (human-authored).

---

## T18 — multi-agent dogfood: who declares harness permissions/agent defs?   🟡 open
Origin: first multi-agent session inside `loom-dev` (2026-06-10) — a bounded
trial of 3 parallel read-only subagents mapping T16's engine touchpoints,
followed by delivering the T16 permissions slice early as repo-tracked
`.claude/` config.

**Trial findings (dogfood feedback).**
- *Fan-out works and cross-validates:* 3 read-only mappers (materialize path,
  containerHome uses, settings.json handling) ran in parallel and independently
  converged on the same hook points (`materialize.go:26-40`,
  `container.go:138,157`) — consistency across blind agents is a useful
  confidence signal serial exploration doesn't give. Cost: each agent re-read
  the T16 thread for context (~3× duplicated orientation reads).
- *Write isolation is mandatory, not optional:* `/workspace/<project>` is a RW
  bind mount shared with the host (single-writer discipline, T13). Standing
  rule adopted: subagents that write each get their own git worktree; the main
  session is the only direct writer in the working tree. Survives today only as
  harness memory + this entry — nothing in loom enforces it (see gap below).
- *Permission friction pre-allowlist is real* (T16's prediction confirmed):
  every read-only fan-out costs prompts until an allowlist exists, which
  pressures a human toward blanket auto-approve — the opposite of
  mechanism-not-trust (ADR-0005).
- *New gap — agent can commit but not ship:* no git credentials and no `gh` in
  the container; `git fetch`/`push` to the https remote fails. An agent can
  branch/commit locally but a human must push and open the PR from the host.
  Same first-class-user break as T15, for the VCS credential instead of the
  Anthropic one. Fold into the T15/credential-acquisition design.
- *New gap — the container can't run its own gate:* `make` was never declared
  in any playbook tier, so the pre-commit hook (`make gate`, RULES §7) failed
  with `make: not found` on the first in-container commit. Deviation taken:
  apt-installed in place (drift), and `make` declared in `loom.yml` so the
  next host-side build converges it. `golangci-lint` was missing for a worse
  reason — undeclared AND undeclarable; that analysis, its out-of-band
  discovery attribution, and the two mechanism gaps it exposes have their own
  thread: T19.
- *Memory model surprise:* the harness now provides a host-side persistent
  memory dir (`~/.claude/projects/<proj>/memory/`) that `verify-loom-dev.sh`
  flagged as a SURPRISE-present — T16's "no memory" assumption is already
  partially stale; ADR-0015 should treat harness-native memory as an input.

**The design question (folds into ADR-0015 / T16).** This session delivered
permissions + agent defs as **repo-tracked** `.claude/settings.json` and
`.claude/agents/*` — versioned, reviewable, and they survive rebuilds for free
because the working tree is the mount (no engine work at all). That carves T16's
scope: what still needs *playbook* declaration vs what rides the repo?
- Repo-tracked covers *per-project, repo-public* policy (allowlist, agent
  defs) — but only for projects that opt in, only after clone, and it cannot
  carry secrets-adjacent or base-wide policy.
- Playbook-declared (T16 lean) covers *user-level* `~/.claude` (hooks, skills,
  base-wide guards, gitconfig identity) and applies to every project the env
  hosts — the two-tier split (ADR-0004) suggests: base-wide guards in the
  playbook, per-project allowlists in the repo.
- Open: should the playbook be able to *reference* repo `.claude/` policy
  (explicit-by-reference, like `rules:`), so `verify` can assert it, rather
  than loom staying ignorant of project-level harness config? And should the
  writer-isolation rule above become declared policy (a guard hook loom
  materializes) instead of a memory-resident convention?

Promote to: input to ADR-0015 (T16 harness-home strategy — this is the
permissions/agents slice of its scope, landed early through the repo); the
push-credential gap to the T15 successor design.

---

## T19 — gate dependency golangci-lint undeclared and undeclarable   ✅ resolved
**Resolution (2026-06-10):** everything operational shipped and verified —
tool fix merged (#19: sourcePolicy + goModule + base playbook + tests),
mechanism (a) merged (#17: claims script probes gate deps the
Makefile-resolution way), lock container-re-pinned (#22), recurrence handled
(stale-binary note above; binaries rebuilt), closing quiz 7/7 with the gate
toolchain engine-guaranteed. What does NOT keep this open: mechanism (b), the
Makefile↔playbook joint check, stays a recorded design in the promote-to by
explicit choice — it graduates via its own queue row when prioritized, not by
holding this thread open. Entry kept for the analysis and (b)'s design.
Origin: **human exploration + out-of-band advisory analysis from the old
environment** (a missing brew formula prompted the trace) — pointedly NOT the
claims script and NOT the gate. That attribution matters: both mechanisms that
exist to catch environment drift were blind to this one, which is the actual
finding.

**The defect (verified in-container, 2026-06-10).** `make gate` hard-fails
without golangci-lint (Makefile resolves it via `command -v` and `lint` exits 1
if absent), yet:
- no playbook tier declared it; it is absent from `loom.lock` and from the
  freshly built `loom-dev`;
- it COULDN'T be naively declared: `goModule()`
  (internal/engine/container.go) mapped only gopls and gitleaks, so a bare
  `golangci-lint` entry would fall to the resolver's default **apt** source
  (internal/resolver/resolver.go `sourceFor`), and debian bookworm ships no
  golangci-lint package — the provision would have broken.
- **Masking:** in `devenv` it existed by accident (`~/go/bin`, hand-installed
  history), so the omission was invisible for the whole pre-cutover period.

**Fix shipped (branch `fix/gate-dep-golangci-lint`).** sourcePolicy gains
`golangci-lint: go-install`; `goModule()` maps it to
`github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`; the base
playbook declares it in `tools:`; resolver + provision-script tests extended
(mirroring the gitleaks mappings). The provision digest changes, so the next
build re-provisions existing containers — expected and non-destructive
(converge, not `--force`; creds volume unaffected); `loom.lock` re-pins on
that host-side build.

**The two mechanism gaps (why nothing caught it).**
- **(a) The claims script doesn't check gate dependencies.**
  `scripts/verify-loom-dev.sh` probes the playbook-declared tool list — a tool
  the playbook never declared is structurally invisible to it. Fix (shipped on
  `feat/verify-loom-dev-claims`): probe golangci-lint in the PRESERVE loop and
  assert every Makefile-resolved gate binary exists.
- **(b) Nothing asserts gate-deps ⊆ playbook-declared tools.** The Makefile
  and the playbook can drift silently — the same class of joint the spec↔FR
  check (`fr-verify`, T3/ADR-0013) exists to guard. Today the joint is
  enforced nowhere: not at build, not in the gate, not in CI.

**Promote to (lean — design only, not built).** Mechanism (b) as a
verify-style joint check: parse the Makefile's required binaries (the
`command -v`-resolved set), assert each is playbook-declared (hence locked and
provisioned). Tiering per ADR-0013 C4, exactly like `fr-verify`: **advisory in
`make gate`, blocking at the merge boundary**, never per-commit. Open
questions for that design: where the "gate deps" set is authoritatively
declared (parse the Makefile vs a small manifest the Makefile and check both
read), and whether the joint generalizes to "any repo-declared workflow dep ⊆
playbook tools" (e.g. the pre-commit hook's own needs — `make` itself was the
same defect, caught the same day; see T18).

**Recurrence (2026-06-10, same day, the stale-$LOOM class).** The post-merge
`loom build` on the Mac silently skipped the expected re-pin: `bin/loom*` were
built at 09:17, the T19 fix merged at 13:55 (`grep -ac golangci-lint` = 0 on
both binaries — the resolver change wasn't in them). Second stale-binary hit
today (the first is this thread's origin note; PR #14 guarded the migration
script against exactly this). Binaries rebuilt from `7bf5988`, grep-verified.
Consider generalizing the migration script's `grep -aq` guard (T17): `build`
could self-check binary-vs-tree currency (embedded commit stamp compared to
the working tree's HEAD) and warn, instead of each script reinventing the
grep.

---

## T20 — container-level egress restriction: the allowlist's arbitrary-code escape hatch   🟡 open
Origin: the auto-mode evaluation (2026-06-10, human-accepted; safe-auto
definition in docs/TEAM.md). The harness permission layer holds at its own
level — deny rules block in all modes, `curl`/`wget`/`WebFetch`/`WebSearch`
are denied — but the allowlist necessarily permits `go test`/`go build`, and
**compiled test code is arbitrary code, including network I/O**. A malicious
or compromised change could exfiltrate (say, the T15 credential file) from
inside an allowed command, and no harness-layer rule can see it. The ADR-0005
design test ("would the guardrails hold if you tried the worst thing?")
**fails at the network layer**; the fix must be container-level mechanism,
not more allowlist entries.

**Option space (no implementation; capture only).**
- **(a) Deny-by-default docker networking, playbook-declared** — a
  `networking:` section (egress allowlist resolved at create). Note the
  **create-time conflict**: provisioning needs broad egress (apt mirrors,
  go.dev, module proxy, installers), and docker cannot change network mode on
  a live container — the same create-time-only class as T13/T14 (labels,
  mounts, volumes). Shapes: provision-then-restrict via network disconnect/
  connect of pre-made networks, or accept recreate-on-policy-change.
- **(b) Egress proxy sidecar** — a proxy container owning the host allowlist;
  the dev container's only route out. Restriction changes without recreate
  (edit proxy policy), and provision + runtime both flow through the same
  audited path. More moving parts; the proxy itself becomes loom-managed
  state.
- **(c) In-container iptables — REJECTED while the agent is root (T10):**
  the agent can undo its own fetters; that is trust, not mechanism
  (ADR-0005). Revisit only after T10 lands a non-root agent, and even then
  prefer (a)/(b) — policy should live outside the box it polices.

**Cross-links.** T10 (root kills option c and weakens everything
in-container); T15 (the credential at rest is the prize an egress channel
exfiltrates — the secret-store/per-use-helper lean shrinks the blast radius
this thread is about); T16 (guard hooks are the complementary *semantic*
layer: they understand intent, the network layer enforces capability — both,
not either); FC-001 (if the lean becomes "icebox the implementation," it
lands there with this entry as the spec seed).

Promote to: an **ADR** (networking policy is architecture — placement of the
egress boundary, playbook schema, the create-time trade) → engine work → FR
per behavior (e.g. "an allowed command cannot reach a non-allowlisted host";
testable with a canary listener in the integration tier).
