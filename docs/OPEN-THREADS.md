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

## T5 — Lockfile doesn't pin what it claims (host-probed `resolved` + no per-tool digest)   🟡 open
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

Promote to: an engine fix (a) + a SPEC-playbook accuracy edit (b), each with an FR
once `verify` covers the lock producer.

---

## T6 — `build --dry-run` mutates (violates plan-semantics contract)   🟡 open
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

Promote to: a bugfix (honor `--dry-run` in `build`) + a regression test asserting a
dry-run leaves container/lock/staging untouched; FR once `verify` covers it.

---

## T7 — `build` converge skips container `$HOME` re-sync on dotfile-only change   🟡 open
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

## T8 — `agents:` are declared but never installed   🟡 open
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
Promote to: an engine capability + a creds decision (possibly an ADR — interacts
with ADR-0005 and the secret-store design); FR once `verify` covers agent install.

---

## T9 — no verb to enter the container (`shell`/`enter`)   🟡 open
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

## T11 — container name `loom-loom-dev` is awkward (doubled "loom")   🟢 recommendation
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
Promote to: an engine change (name template + labels) + a note in ADR-0001 (naming/
namespacing convention); FR once covered.

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

**Exit criteria before deleting `devenv` (not just archiving).**
1. T8 — `loom-dev` installs the agent + has working credentials.
2. T9 — an entry path exists (verb, or documented `docker exec -it`).
3. The loom **dogfood loop runs in `loom-dev`**: `make gate` (unit tier; go present)
   works; integration tier stays on **CI** (already true) or `loom-dev` gains docker
   access (DinD/socket — likely FC-001) if local integration is wanted.
4. Credentials/config that `devenv` got via Mac bind-mounts are reproduced in
   `loom-dev` (T8 creds decision).

**Counter-argument considered & rejected:** "keep `devenv` to compile the loom
engine (Mac has no go)." Rejected because `loom-dev` already declares `go@1.26`, so
it can build the engine and run the unit gate itself; integration is a CI concern.
Once 1–4 hold, `devenv` adds nothing.

**Sequencing:** archive now (stop investing in it; document it as legacy), remove
after T8+T9 land and the dogfood loop is proven in `loom-dev`.
Promote to: an ADR recording the single-dev-container model + `devenv` retirement
(human-authored, since it touches the env/topology decision), once exit criteria met.
