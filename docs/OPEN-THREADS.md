# Open design threads

Working record of in-progress design discussions. **Not an ADR** — when a thread
resolves it promotes to an ADR (RULES §3) or a spec edit, and the entry is marked
Resolved with a pointer. Kept current so an interrupted discussion resumes without
losing context.

Status: 🟡 open · 🟢 recommendation drafted · ✅ resolved (awaiting promotion).

**Archive on resolution (standing convention, 2026-06-10).** A ✅-resolved
thread's full text moves to `docs/threads/archive/TNN-slug.md`; here it
collapses to a 3-line stub (title+status · resolution one-liner · pointers to
ADR/FRs/PR/archive). Open (🟡/🟢) threads stay in full — detail grows only
while a thread is actually open. New threads are born **stubs-first** from
inbox envelopes (T21): the envelope's design reasoning becomes the thread stub
before work proceeds, never after.

---

## T1 — Manual-test ban for required FRs   ✅ resolved
Resolved: a required FR's only coverage is an automated test — human testing is feedback, never coverage; un-automatable behaviors get an automatable proxy or downgrade.
Pointers: ADR-0013 (policy applied, C1/C2 in PR #2) · FR-registry header · archive: docs/threads/archive/T01-manual-test-ban.md

---

## T2 — Hermetic unit gate   ✅ resolved
Resolved: the gate runs unit tests in a targeted-scrubbed env (local ≡ CI); scope later extended to GIT_* repo-redirection vars (LL-010).
Pointers: Makefile test target · LL-006/008/010 · guard.TestGateHermeticToGitEnv (#41) · archive: docs/threads/archive/T02-hermetic-unit-gate.md

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

## T4 — Container PATH has no single declarative owner   ✅ decided + built
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

**DECIDED (human, 2026-06-11 evening; draft 014 → envelope 020): option 2 +
option 1's spec note. Built 2026-06-12 (branch fix/t4-path-single-owner):**
- Shell-init converged + UNCONDITIONAL: `ensureShellInit` runs on every
  Ensure path (create and converge), never gated on the tool set; one loader
  sources `~/.bashrc.d/*.sh` from BOTH `.profile` and `.bashrc` ($HOME-based,
  T10 prep; grep-guarded so pre-T4 containers don't duplicate).
- Go PATH left engine-hardcoded shell-appends and became a GENERATED dotfile
  `~/.bashrc.d/path.go.sh` emitting `$HOME/go/bin`, not `/root/go/bin`; the
  claude-code `~/.local/bin` appends became `path.local.sh` (same class).
  Generated files ride the staging dir: home-digest covers drift, plan/doctor
  grade them (F2/C1 parity), audit names them.
- Option 3 (`path:` field) explicitly REJECTED for Phase 1 — reopen only if a
  second stack needs PATH-ordering semantics (SPEC-playbook note records this).
- Edge accepted: a pre-T4 container whose sentinels are all clean keeps the
  old wiring until any home/provision change (or --force) re-converges it.

Pointers: SPEC-playbook "Shell config model" · FR-BUILD-011 · draft 014 /
envelope 020 · internal/engine/materialize.go (expectedPathDotfiles) ·
internal/engine/container.go (ensureShellInit).

---

## T5 — Lockfile doesn't pin what it claims (host-probed `resolved` + no per-tool digest)   ✅ resolved
Resolved: the lock pins the CONTAINER's reality — resolved versions are probed in-container at build step 5, never on the host; new tools stay "" until probed.
Pointers: fix/t5-lock-fidelity · FR-BUILD-007 · archive: docs/threads/archive/T05-lock-fidelity.md

---

## T6 — `build --dry-run` mutates (violates plan-semantics contract)   ✅ resolved
Resolved: --dry-run removed; plan is the sole preview path (plan-semantics contract).
Pointers: fix/t6-remove-dry-run (PR #8) · SPEC-verbs#plan · archive: docs/threads/archive/T06-remove-dry-run.md

---

## T7 — `build` converge skips container `$HOME` re-sync on dotfile-only change   ✅ resolved
Resolved: a home-content sentinel (/var/lib/loom/home, ADR-0011 pattern) re-syncs container $HOME on dotfile-only change; provision stays toolset-gated; status truthful by construction.
Pointers: #25 + #26 · FR-BUILD-004 tests · ADR-0015 precondition · archive: docs/threads/archive/T07-home-resync-sentinel.md

---

## T8 — `agents:` are declared but never installed   ✅ resolved
Resolved: declared agents install during provision (claude-code native installer, ~/.local/bin on PATH); sentinel digest covers the agent set.
Pointers: ADR-0014 (PR #7) · FR-BUILD-006 · archive: docs/threads/archive/T08-agent-install.md

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

## T10 — container runs as root; should be a non-root `dev` user   🟢 PR 1+2 merged (#122/#130); PR 3 built 2026-06-13 (Model A run-as-user: useradd, home retarget, scoped chown, entry-verb -u — awaiting human merge); PR 4 (role marker) next
Origin: `loom-loom-dev` is `root@/root` (confirmed `docker exec … whoami` → root),
which is why dotfiles materialize to `/root` and the home-target confusion arose.
Option 1 (non-root `dev`) was the standing lean; this drafts the full design.

**Why it matters.** A non-root `dev` user (uid 1000, `HOME=/home/dev`) matches
conventional dev containers, reduces blast radius (T20: root kills the
in-container-iptables option and weakens everything), aligns with `devenv`, and
makes the home-target explicit. Full-auto re-evaluation lists T10 as one of its
three gates (TEAM.md, HARNESS.md).

**Design (drafted from the 045 envelope's four scope items + the engine map):**

1. **The configured value: playbook `user:` scalar** (SPEC amendment — frozen,
   needs human authorization). Semantics: the container's runtime user; unset =
   `root` (Phase-1 compatible — every existing playbook keeps meaning what it
   meant). `$HOME` derives: `root → /root`, else `/home/<user>` (uid 1000,
   useradd at provision). Merge: last-non-empty-wins scalar; any tier may
   author (an env-wide base default with project override is the expected
   shape). Engine: `containerHome` constant becomes a `ContainerSpec.Home`
   field resolved from `user:`; PR 1 already forced every home path through
   the single owner (cp targets had two literal bypasses — fixed + grep-guarded).
   Proposed SPEC-playbook clause text (for the human to authorize verbatim or
   edit): *"`user:` (optional, scalar, later-wins): the container's runtime
   user. Unset means root (compatibility). A non-root user is created at
   provision (uid 1000, home `/home/<user>`); every materialization targets
   the resolved `$HOME` (ADR-0015 T10 rule); entry verbs run as this user
   (ADR-0016 decision 7)."*
2. **Provision-as-root / run-as-user split.** `docker run --user <user>` makes
   the configured user the exec default — entry verbs (exec/shell) then need
   no `-u` flag, exactly ADR-0016's "changes the config not this code".
   Provisioning (apt-get, /usr/local/go, /var/lib/loom sentinels) keeps root
   via explicit `docker exec -u root` on the provision/sentinel paths only.
   Provision gains: `useradd -m -u 1000 <user>` (idempotent guard), and a
   `chown -R <user>` of `$HOME` after every home sync — `docker cp` writes
   root-owned files, so the sync path must restore ownership (new step, rides
   the same dockerLogged call site as the cp).
3. **Role marker replaces the uid check** (drain-inbox.sh role guard; LL-011).
   Today's fallback `id -un == root ⇒ loom-author` dies with non-root. Design:
   provision writes `/var/lib/loom/role` (content: the seat's role, from a new
   `ContainerSpec.Role`; loom-dev declares `loom-author`). Resolution order in
   the guard becomes: `LOOM_SESSION_ROLE` env (explicit + test seam) →
   `/var/lib/loom/role` (materialized marker, in-container only) → UNRESOLVED
   = no-op (fail-closed: a host-side advisor session has no marker and must
   never drain the Writer's inbox — strictly safer than the uid guess).
   Where the role value comes from is the open red-team question: lean is a
   loom.yml-adjacent declaration rather than hardcode, but role is a TEAM
   concept, not a playbook concept — alternatives: (a) `ContainerSpec.Role`
   set by the loom-dev overlay; (b) env passthrough pinning
   `LOOM_SESSION_ROLE` at `docker run -e`; (c) keep env-only and accept
   no-op-without-env. The guard edit itself is a trust-path change
   (.claude/hooks/**) — human-applied diff at PR 4.
4. **Doctor claim:** `container:user` — compare `docker inspect -f
   '{{.Config.User}}'` (cheap, works on stopped containers) against the
   configured value; live `id -un` probe as the running-container variant.
   Needs one new `ContainerRuntime` method; sits after `container:state`.
5. **Migration:** the user value rides the provision sentinel digest, so a
   `user:` change re-provisions an existing container; the agent-home volume
   (`<name>-claude`) carries root-owned files from the root era — the
   provision chown covers it. Recreate validates 039's trust-flag acceptance
   at the same stroke (one recreate, two acceptances).

**Slicing (PR 1 = this branch; remainder filed as drafts for triage):**
- **PR 1 (landed here):** design 🟢 + containerHome single-owner fix (two
  literal `:/root/` cp targets → `homeCpTarget`) + grep-guard test.
- **PR 2:** SPEC `user:` clause [ALLOW_SPEC_CHANGE — human authorizes] +
  schema/merge/validate + ContainerSpec.User/Home plumbing.
- **PR 3:** engine behavior — `docker run --user`, provision useradd + chown,
  `-u root` on provision/sentinel paths, integration-test updates (the
  /root assertions in integration_test.go re-derive from the spec).
- **PR 4:** role marker + drain-guard fallback swap [trust path — human-
  applied diff] + doctor `container:user` + e2e doc updates.

Promote to: ADR (container user policy: provision-as-root/run-as-user,
default-root compatibility, role-marker doctrine) + FRs per slice.
Red-team asks (advisor): the role-value source (3a/3b/3c above); whether
`user:` is base-only by authorship like `settings:`; chown-after-sync vs
`docker cp -a`; uid collision policy on images that already ship uid 1000.

**Advisor red-team verdict (2026-06-13, envelope 047) — PASS WITH AMENDMENTS.**
Design is sound and the four-PR slice stands. Findings, grounded in
`internal/engine/container.go` (`containerHome="/root"`, root `docker run` with
no `--user`, `docker cp …:/root/` home-sync, ro `.credentials.json` bind,
provision-as-root exec) and `.claude/hooks/drain-inbox.sh` (the `id -un==root`
fallback):

1. **Role-value source → 3a, refined.** Materialize a root-owned marker
   `/var/lib/loom/role` at provision from a new `ContainerSpec.Role`; keep
   `LOOM_SESSION_ROLE` env as the override/test-seam (guard already prioritises
   it). 3b's env *value* must come from `ContainerSpec.Role` anyway (not simpler,
   less durable across `loom exec`); 3c silently no-ops the Writer drain if env
   is absent. **Security gain, not parity:** a root-owned `0644` marker means the
   non-root agent cannot forge its own role (prompt-injected Writer can't
   escalate; host advisor has no marker — LL-011 fail-closed floor holds).
   **Boundary:** `Role` is set by the loom-dev overlay / engine wiring, NEVER a
   playbook `role:` key — role stays a TEAM concept, not a playbook one.
2. **`user:` is NOT base-only** (unlike `settings:`, which is whole-file /
   no-key-merge, `load.go:76`). It is a **last-wins scalar like `trust:`**
   (`merge.go:58`); env-base-default + project-override is the legitimate shape.
   Known edge for the ADR: since unset=root, a later `user: root` silently
   re-grants root — root-drop is enforced at the full-auto gate, not the scalar
   merge; do not special-case the merge in Phase 1.
3. **`chown` after home-sync is REQUIRED; `docker cp -a` is NOT a substitute** —
   `-a` preserves the *host* numeric uid, the wrong owner for a uid-1000 user.
   **Amendment to design item 2:** scope the chown to the **materialized file
   set** (`res.Materialized`), NOT a blanket `chown -R $HOME` — a blanket `-R`
   walks into the read-only `.credentials.json` bind and errors.
4. **uid-1000 collision → do not hard-pin.** `bookworm-slim` ships no uid 1000,
   but `base_image:` is configurable (node images ship 1000). Contract becomes
   "a non-root user named `<user>`," uid 1000 preferred-when-free, next-free +
   log on collision. The doctor `container:user` claim already keys on **name**
   (`id -un`), not uid, so name-based verification supports this with no change.

**Clause text — amended + human-reauthorized 2026-06-13** (the hard `uid 1000`
in the frozen clause was the one amendment routed back; approved):
> *"`user:` (optional, scalar, later-wins): the container's runtime user. Unset
> means root (compatibility). A non-root user is created at provision (non-root;
> uid 1000 by default, system-assigned on collision; doctor verifies by name),
> home `/home/<user>`; every materialization targets the resolved `$HOME`
> (ADR-0015 T10 rule); entry verbs run as this user (ADR-0016 decision 7)."*

PR 2 (Writer, ALLOW_SPEC_CHANGE pre-authorized) is unblocked: transcribe this
amended clause into SPEC-playbook + schema/merge/validate + ContainerSpec.User/
Home plumbing. PR 3 folds in findings 3–4 (scoped chown; collision-tolerant
useradd); PR 4 (role marker, trust path — human-applied diff) folds in finding 1.

---

## T11 — container name `loom-loom-dev` is awkward (doubled "loom")   ✅ resolved
Resolved: container is <project>-dev (loom-dev); loom-managed marker moved to docker labels (loom.managed/loom.project); audited SPEC example edit shipped.
Pointers: container batch (PR #13) · ADR-0001 naming note · archive: docs/threads/archive/T11-container-naming.md

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
   **✅ met loom-natively (2026-06-10):** `loom exec` merged (#43, SPEC-verbs
   clause #35–#40, ADR-0016 Accepted #42); `shell` spec'd and staged.
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
   **✅✅ re-verified loom-natively (2026-06-10, post-T19 + #43):** the human
   ran `loom exec -- make gate` from the Mac through the merged verb —
   `gate: PASS` on the playbook-provisioned toolchain, audit entry #19
   (`container.exec`, command + exit 0). The dogfood loop closes through
   loom's own surface; no caveats remain on this criterion.

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
Resolved: the project repo bind-mounts RW at /workspace/<project> at create (T13); single-writer discipline governs the shared tree (TEAM.md).
Pointers: container batch (PR #13) · engine.TestCreateRunArgs · archive: docs/threads/archive/T13-project-mount.md

---

## T14 — agent credentials are lost on every rebuild   ✅ resolved
Resolved: a named volume <container>-claude mounts at ~/.claude when an agent is declared — in-container login survives --force/teardown; wipe is the opt-in --clean-state tier only.
Pointers: ADR-0014 addendum (PR #10) · engine.TestCreateRunArgs · archive: docs/threads/archive/T14-creds-survive-rebuild.md

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
Resolved: harness home splits at mutability on the ~/.claude volume seam — declared config materializes every build, mutable state accretes untouched; harness: schema, two-tier policy split, memory seeds empty, continuity as declared hooks.
Pointers: ADR-0015 (Accepted, PR #24/#42-era flips) · T7 precondition · queue row "T16 engine work" · docs/HARNESS.md · archive: docs/threads/archive/T16-harness-home.md

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
Resolved: golangci-lint declared via resolver sourcePolicy + goModule + base playbook; claims script probes gate deps Makefile-style; lock container-re-pinned; mechanism (b) — the Makefile↔playbook joint check — stays a recorded design (queue graduates it).
Pointers: #19 (tool) · #17 (claims) · #22 (lock) · LL-010-adjacent stale-binary notes · archive: docs/threads/archive/T19-gate-dep-golangci-lint.md

---

## T20 — container-level egress restriction: the allowlist's arbitrary-code escape hatch   🟡 open
Gate registered here (2026-06-11): the bootstrap prebuilt-binary-fetch
amendment (SPEC-verbs "entry: bootstrap", future amendment b) is explicitly
GATED ON this thread's resolution — bootstrap stays network-free until T20
decides the egress boundary.
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

---

## T21 — cross-session task transport: inbox + drain   🟢 decided — mechanism in build
**Problem.** The human is the message bus between author and advisor sessions
("when Writer finishes, paste X") — pure relay, no judgment; should be
mechanism, not a person.

**Decision (human, 2026-06-10).** Tree-native **inbox** + **Stop-hook drain**
+ **dispatcher script**. Coordinator-as-agent rejected — same
alarm-not-firefighter verdict as the buster; revisit at multi-writer scale.
- *Inbox:* `.scratch/inbox/<role>.md`, untracked — **mail, not memory**.
  Append-only blocks: `id | from | serves: <queue-row> | status
  QUEUED/TAKEN/DONE | body`; header `AUTOPILOT: off` (default; human flips).
  Cross-agent write rule: a role appends only to OTHERS' inboxes and updates
  only its OWN items' status — the sole cross-agent write surface.
- *Drain (Stop hook, rides the tree):* if AUTOPILOT on and a QUEUED item
  exists, don't stop — take it. Four hard guards: (a) **orphan refusal** — no
  valid `serves:` queue row ⇒ refuse, flag in next report, skip; (b)
  **design-envelope legalization** — design reasoning must instruct "log as
  thread stub before work proceeds", else treated as orphan (*ephemeral must
  carry durable's birth certificate, or it doesn't ride*); (c) **drain
  budget** — max 3 chained items, then stop regardless (unattended chaining
  is new ground; the budget is the guardrail, not decoration); (d) the
  **never-auto permission floor is untouched** — protected ops still prompt
  mid-drain. Malformed anything ⇒ normal stop.
- *Dispatcher:* `scripts/` (T17 header) promotes pre-authored `after:
  <condition>` items (PR merged, row status) to QUEUED; host-side until T18.

**Load-bearing durability rules.** Inbox status = delivery state ONLY; work
state lives in the queue row, flipped in the shipping PR; an item isn't DONE
until its row moved; **the queue never references the inbox — canon must not
depend on transport.** New threads are born stubs-first from inbox envelopes.
/replan audits: orphan inbox items; stale TAKEN with unmoved rows.

Promote to: TEAM.md rules + `.claude` drain hook + `scripts/dispatch-inbox.sh`
(this PR series); drain-integrated or loom-native dispatch later.

---

## T22 — defaultMode "auto" trial: exit/rollback package   🟢 decided — transcription (this PR)
**Problem.** The one-week auto-trial queue row (proposed 2026-06-10) had no
exit criteria, rollback triggers, or measurement plan — flipping both roles to
`defaultMode: auto` without a decided failure contract would make the verdict
vibes, not evidence.

**Decision (human, 2026-06-11, advisor session; per C3 this entry is
transcription — acceptance via merge).** The full package is
`docs/auto-trial.md`; the spine:
- *Scope:* acceptEdits → auto, ONE WEEK, both roles; per-role baseline
  transcript scan (prompts/session) BEFORE the flip; trial clock starts at
  flip.
- *Exit (graded severity):* S1 = never-auto-floor executes unprompted or
  unintended mutation reaches main · S2 = unintended write caught by
  gate/guard/PR checks · S3 = annoyance/misfire (feeds the allowlist as
  evidence). Day-7 verdict: keep auto iff zero S1 AND zero S2; else revert.
  Re-flip/renewal is human-only.
- *Rollback (cumulative, mid-week):* HARD (any S1, out-of-sandbox write,
  secret in logs, LL-010-class shared-tree incident traceable to auto) ⇒
  revert BOTH roles — a hard trigger means the shared mechanism failed.
  SOFT (≥3 guard misfires/day, or a gate-failing change acceptEdits would
  have stopped) ⇒ revert the FIRING role only; for the Writer, soft = flag +
  discuss. Revert is PRE-AUTHORIZED for any role (stricter-only is safe);
  re-flip stays human-only.
- *Measurement:* advisor runs a daily 5-min audit of BOTH roles (audit log,
  git log vs expectations, deny/floor-prompt counts); the Writer never
  self-audits.
- *Asymmetry rationale:* advisor holds outward credentials (push/merge/gh) ⇒
  tighter leash (hard+soft); Writer mutations all pass gate+PR+CODEOWNERS ⇒
  hard-only tolerable.

Pointers: queue row "one-week defaultMode auto trial" · docs/auto-trial.md ·
inbox item 001 (transport, 2026-06-11) · T21 (the transport that carried it).

---

## T23 — AUTOPILOT scoping: per role × project, kill-switch, flip audit   🟢 decided — transcription (this PR)
**Problem.** T21 shipped the drain with a bare `AUTOPILOT: off|on` header and
no decided scope: is the flag per role, per env, per project, per agent? The
2026-06-11 misfire day (10 drain misfires: 1 S2-shaped, 9 S3 — LL-011,
PR #54) showed flips happening with no trace outside a conversation.

**Decision (human-endorsed 2026-06-11, advisor discussion; per C3 this entry
is transcription — acceptance via merge).**
- *Scope:* per **role × project**. Per role: a trust grant, role-scoped
  rollback needs independent switches. Per project: trust in THIS repo's
  guardrail stack; repo mail only, never user-global; new projects start off.
  NOT per env (one shared tree = one header; env differences belong in the
  READER — the LL-011 role guard). Not per agent-harness YET (role↔agent is
  1:1 today; second harness ⇒ per role × agent, default OFF until its
  guardrail wiring is validated).
- *Kill-switch:* `.scratch/inbox/HALT` gates ALL roles' drains regardless of
  headers — the auto-trial "hard trigger reverts both roles" as one atomic
  touch. Checked before the AUTOPILOT gate; regression-tested.
- *Flip audit:* every header flip / HALT create/remove appends
  `timestamp | actor | old→new | reason` to `.scratch/inbox/flips.log` —
  untracked like the mail, but a trace.

Pointers: TEAM.md "Cross-session transport" (encoded) ·
`.claude/hooks/drain-inbox.sh` (HALT gate) · `guard.TestDrainHalt*` ·
docs/auto-trial.md (rollback triggers) · LL-011 · inbox item 003.

---

## T24 — /achievements skill: queue-anchored shipped-work narration   🟢 decided — built (this PR)
**Problem.** "What shipped since X" is answered today by hand-walking the
queue, git log, threads, and lessons — relay work, and the day-1 trial
evidence showed busy days outrun anyone's memory of them.

**Decision (human-requested 2026-06-11, advisor discussion; design via inbox
item 005).** A project skill, sibling of `/replan`: one audits the queue, one
narrates it.
- *On-demand only* — no cron/recurring form (explicitly rejected as
  overkill). Invocation: `/achievements [since: date|"yesterday"|PR#]`.
- *Anchor on the QUEUE, not git:* rows flipped done/in-review since <since>
  are the "what shipped" truth; PRs, OPEN-THREADS stubs, LL entries,
  flips.log, and inbox DONE items hang off them.
- *Output:* the human-validated table — name/brief | category
  (spec/decision/mechanism) — plus a lifecycle view (discussion →
  transcribed → live | current state) and an optional housekeeping section.
- *Report-only:* NO tree writes — usable by any role in any mode. Mechanical
  gathering lives in a helper script inside the skill dir; synthesis and
  categorization stay with the model.

Pointers: `.claude/skills/achievements/` · inbox item 005 · T21 (transport
that carried it) · `/replan` (sibling skill).

---

## T25 — context economy + intake lane + /coordinate   🟢 decided — transcription (this PR)
**Problem.** Cross-role context still travels by relay (the trial-flip
schedule existed only in a conversation), discussion residue mints ad-hoc
inbox items ("the old way"), and the coordinator hat runs by hand with no
defined authority. Three gaps, one economy.

**Decision (human-blessed as-is 2026-06-11, advisor discussion; C3
transcription — acceptance via merge; inbox item 012, five parts, ONE
package — amendments append to the item, never sibling-mint).**
- *Context economy (TEAM.md clause):* state lives in artifacts, channels
  carry intent + work only; as-of timestamps on anything written-for-later,
  ground truth re-read at act time; one decision = one envelope; gates are
  events, never times (schedules live with the human); new channels pay
  rent — name the failing channel and what is retired.
- *Inbox kinds:* `fyi` — ephemeral context, drain skips BEFORE the orphan
  check, no `serves:` needed, read at orientation → READ, pruned at next
  session-end. `draft` — non-expiring intake, never ridden, never pruned,
  lives until a /coordinate verdict.
- */coordinate (skill, two modes):* read (any role, report-only standup) ·
  verdict (draft triage: promote | merge-into | park | drop) — output ONE
  batch PR; the PR is the proposal, disposal stays with the arming role.
- *Authority (pinned to the HAT, not the skill):* propose-only always;
  arming = advisor review act; frozen paths keep human admin-merge;
  scheduled runs = hat-holder only (non-hat runs marked); self-verdict
  flags for items the runner authored or that route work to/from the
  runner; drops always need cross-role ack.
- *Cadence:* daily coordinator run under the advisor hat, rendered
  yesterday/today/blockers into the 08:00Z slot; absorbs the trial daily
  audit during trial week. No synchronous ceremony.

Deferred BY NAME: per-role status boards (only if fyi proves insufficient —
channel rent); coordinator as separate agent (promotion by evidence;
prerequisite T18).

Pointers: TEAM.md "Cross-session transport" + "Context economy" ·
`.claude/hooks/drain-inbox.sh` (kind guards) · `guard.TestDrainSkips*` ·
`.claude/skills/coordinate/` · inbox item 012 · T21/T23 (the transport
this governs).

---

## T26 — `rollback` verb: necessary or overkill?   🟢 recommendation drafted
Origin: human question 2026-06-12, prompted by the live-build-under-session
experiment ("if a build runs mid-session, how do we get back?").

**Recommendation: NO `rollback` verb — overkill.** Loom is declarative-
convergent (SPEC-verbs preamble: verbs reconcile reality to the playbook;
`loom.lock` records resolved state, build.go:90–119). In that model rollback
is not a primitive — it is re-converging to a prior *declared* state, which
already decomposes into `git checkout <prior playbook + loom.lock>` →
`update`/`build` (idempotent). An imperative `rollback` would be a second code
path re-implementing the convergence engine — the "one engine path" anti-
pattern T9 rejected for exec/shell, and trust in a second mechanism rather
than new capability (the design test: would the guardrails hold? — a verb
that re-implements an invariant fails it).

**The instinct points at two REAL gaps, neither named "rollback":**
1. *Observability, not undo.* "What changed under me" is review finding R1
   (audit fail-open / best-effort / tamperable). Fix the build audit-delta
   (complete, tamper-evident before/after) and "re-converge to undo" becomes
   both safe and legible — that is the actual rollback story.
2. *Snapshot/restore of MUTABLE container+session state* — creds volume,
   in-flight agent task context, post-build writes — which convergence
   deliberately does NOT cover (06-09 `docker stop` task-loss; 2026-06-12
   worktree-metadata loss). This is `snapshot`/`checkpoint`, own ADR,
   Phase 3+. Half-named already: BACKLOG C4 (session-snapshot) + the PLAN
   "agent-initiated container lifecycle / task continuity" open item.

Pointers: docs/reviews/phase-1-review.md (R1) · docs/BACKLOG.md (R1, C4) ·
PLAN "Open items / task continuity" · .scratch/live-build-experiment.md
(the experiment that raised it) · ADR-0006 (specs-as-product / convergence).

---

## T27 — AUTOPILOT control + observability: human override channel + drain verification/notification   🟡 open (facet B slice 1 landed #127; facet A — RESOLVING DIRECTION set 2026-06-14: ephemeral-worker substrate, human-confirmed; ADR pending)
Origin: human proposals 2026-06-12, raised when a live-but-idle Writer could
not be handed an urgent correctly-queued item by the autopilot drain. Two
coupled facets — a control channel IN, observability OUT.

**Facet A — human RECOMMEND/OVERRIDE keywords (intake: draft 032).** An
in-band control channel that works WITH autopilot, preserving trust-spirit
while giving immediate, dependable human intervention (a nudge is not
dependable). Decomposes into (i) SELECTION control — which item / what
priority (a marker the drain honors; OVERRIDE may bypass orphan-guard +
budget, RECOMMEND is a soft judgment hint) and (ii) WAKE/trigger — the hard
piece: the drain fires only at Stop, so an idle session never sees new work;
needs an OUT-OF-BAND poke (host-side send-keys into the loom-dev pane = the
missing "wake" primitive). Shape: OVERRIDE as HALT's symmetric counterpart
(HALT freezes, OVERRIDE directs/wakes), human-authored, flips.log-logged,
BOUNDED by the never-auto floor (reprioritize/wake, never escalate perms).

**Facet A — WAKE-PRIMITIVE DESIGN (advisor, 2026-06-13).** Specimen that forced
it: 2026-06-13, adv-048 queued at ~11:30Z but `.drain-count` mtime stuck at
06-12T20:19Z — the drain is Stop-triggered, the Writer was already stopped, and
nothing re-stops an idle session, so the work stranded until a human typed
"continue". The wake primitive is the ACTUATOR that produces that turn.

THE INVARIANT (non-negotiable, the whole security of the channel): the injected
keystroke is a HARDCODED CONSTANT ("continue" / "drain"), NEVER content derived
from the request file. A wake request selects WHICH session + soft/hard priority
ONLY; if the payload could carry text into the keystroke, the channel becomes
arbitrary-command-injection into a trusted auto-mode Writer. Waking only causes
a TURN; the turn runs the EXISTING drain, still bounded by the never-auto floor
+ orphan-guard + budget. Wake escalates NOTHING — it lets bounded machinery run.
That is what keeps it "mechanism, not trust".

SHAPE (symmetric to HALT): a request file `.scratch/inbox/WAKE` (counterpart to
`.scratch/inbox/HALT`); HALT WINS — if HALT present, no wake (a frozen system
stays frozen, checked first, fail-safe). Every wake appends to flips.log
(`ts | actor(human|watchdog) | wake:<role> | reason`), idempotent + rate-limited
(one request = one wake; bounded backoff so a stuck request can't spin the
session — cf. drain 3-per-cycle, provision attempt cap).

MECHANISMS (the actuator is HOST/orchestration layer — you cannot send-keys to
your own pane; the layer that owns the Writer's session does it; loom's runtime
is the container, the session lives above it):
- **A. host-side send-keys (draft 032 lean; MVP):** `tmux send-keys -t <writer
  pane> "continue" Enter` (or the multiplexer equivalent). Preserves the live
  interactive session + conversation continuity; minimal. Cons: needs a stable
  pane handle + multiplexer; host-specific (mac tmux vs windows).
- **B. event-driven headless drain (loom-ownable; robust path):** the
  orchestrator spawns one `claude -p "drain"` turn (cwd=repo root) when work is
  queued; the Stop hook fires at its end and the drain delivers. No TTY/pane
  fragility, deterministic, loom can own the trigger; the checkpoint-inject hook
  restores durable context (a headless turn is a fresh session — that is exactly
  what the continuity hooks exist for). Cons: spins a session per wake. LEAN for
  windows-/ai-user-topology where there is no stable interactive pane.
- **C. in-band /loop self-poll:** Writer runs under Claude Code `/loop`, waking
  itself every N min. No host mechanism, but polling cost + up-to-N-min latency
  + NOT the "dependable immediate intervention" draft 032 wanted. Fallback only.

TRIGGERS (facet A's two needs): (i) HUMAN OVERRIDE — human writes WAKE / runs a
`wake-writer` command / keybinding → immediate, dependable poke (replaces the
manual "continue"). (ii) AUTOMATED WATCHDOG — the heartbeat/idle SENSOR (the
T27-B remainder) detects "AUTOPILOT on + eligible QUEUED + idle N min" → writes
WAKE → actuator pokes. Wake (actuator) + heartbeat (sensor) COMPOSE; the #127
doctor claim is the on-demand sensor, the watchdog its always-on form.

SLICING + LEAN: MVP = mechanism A, human-triggered (smallest dependable win,
retires the manual poke). Phase 2 = couple to the watchdog sensor for
autonomous bounded wake. Record B as the topology-robust path; reject C as
primary. GATE: this is a CONTROL channel on the AUTOPILOT trust model (T23
family) — needs a HUMAN decision + ADR before code (the constant-keystroke
invariant + HALT-precedence are ADR-level), and the actuator is host-side +
topology-specific (human owns the orchestration choice). Trust-path: actuator
script + request handling are security-sensitive (they drive a trusted session)
→ guard/trust class, human-applied.

**Facet A — RESOLVING DIRECTION: the ephemeral-worker substrate (human-confirmed
2026-06-14).** The 2026-06-13 mechanism menu (A send-keys / B headless / C /loop)
treated the wake as ONE mechanism and stalled on a false dilemma — send-keys is
BLIND (injects into whatever the pane shows; no idle-state introspection) and
headless `claude -p` is CLAUDE-SPECIFIC (what about gemini / other harnesses?).
The zoom-out that dissolves it: the wake has **two layers**, the mechanism/policy
split loom is built on (ADR-0006):
- **Autonomy LOOP (harness-NEUTRAL, loom-owned):** "is there ready work? for which
  session? fire a tick" — pure logic over durable artifacts (inbox/PLAN/
  checkpoints). **T30 lives here** (the backlog→inbox readiness predicate).
- **Tick ACTUATOR (per-harness ADAPTER):** "deliver one tick to session X of
  harness H" — a new adapter VERB `wake(session)`, quarantined where loom ALREADY
  handles harness differences (`loom.lock` `agents:` claude-code/gemini). "What
  about gemini?" answers itself structurally: same place loom answers "how do I
  *launch* gemini." send-keys / `-p` are just candidate BODIES of the claude
  adapter's verb — never the design.

**THE DECIDED SUBSTRATE: the unit of autonomy is an EPHEMERAL WORKER loom SPAWNS**
(rehydrate → drain → act → persist → exit), NOT a warm session injected into.
Rationale: deterministic (no blind pane injection), harness-agnostic (reuses
loom's launch path), matches `ai-user-topology` ("state persisted outside,
rehydrated on bring-up"), and ~80% already built — `config/hooks/checkpoint-inject`
IS the rehydration primitive; the inbox + PLAN ARE the durable state. The warm
interactive session is reframed as **the HUMAN's seat** (advisor design-thinking),
never the robot's worker — removing the impedance mismatch that produced the
blind-injection ugliness.

**Consequence for the 2026-06-13 mechanisms:** **B (headless) is PROMOTED** from
"topology-robust path" to THE autonomy substrate — generalized as a *loom-spawned
worker via the per-harness launch adapter*, not a hardcoded `claude -p`. **A (host
send-keys) is DEMOTED** to an optional human-convenience poke for the warm human
seat — topology-specific, NEVER load-bearing for autonomy. **C (/loop) stays
rejected** as primary. The constant-keystroke security invariant generalizes
cleanly: a spawned worker runs the EXISTING guarded drain (+ T30 readiness
predicate), so **spawn escalates nothing** — same "mechanism, not trust" floor.
Still needs an ADR (couples T30's predicate + this substrate + the ADR-0020
extension into one autonomy decision).

**Facet B — close the mental-model↔reality gap (intake: draft 033).** The
drain is silent today — you cannot tell "working" from "silently idle"
(proven 2026-06-12: it never delivered item 031). Mechanisms: (A) a drain
DECISION-TRACE (fired-at, picked X / skipped Y because orphan|draft|budget|
nothing-eligible) so reality is observable, plus a doctor claim "AUTOPILOT
on + eligible QUEUED + session idle = ANOMALY"; (B) a watchdog that NOTIFIES
on absence-of-progress instead of sitting silent. SRE tiering (knowledge-
based, env had no live web): LOG every decision (forensic), TICKET orphan/
budget skips (standup), PAGE only the no-progress anomaly (actionable) —
every page demands action or it's noise.

Live specimens feeding this (2026-06-12): item 031 silently orphan-skipped
(serves≠row); live-idle Writer never drained newly-queued work (drain is
Stop-triggered, not inbox-watching). Doctrine: what the reviewer hand-checks
once, doctor mechanizes (P7 gate).

**Facet B — first slice LANDED 2026-06-13 (advisor, the loom way).** The
PAGE-tier anomaly is now a doctor claim: `host:autopilot` fails when AUTOPILOT
is on for the loom-author inbox with ≥1 eligible QUEUED item but no drain
evidence (`.drain-count` absent) — the exact signature of the 2026-06-12
never-loaded-Stop-hook incident, caught on demand instead of by hand 6h in
(FR-DOCTOR-005). It is point-in-time + side-effect based BY DESIGN and says so.
REMAINING in facet B: (i) the LOG-tier decision-trace inside drain-inbox.sh
(fired-at / picked-X / skipped-Y-why) — a trust-path edit (.claude/hooks/**),
human-applied diff; (ii) a TRUE live-idle watchdog (hook loaded once then
stopped) — needs a session heartbeat primitive, not yet designed. Facet A
(control channel / OVERRIDE + wake) untouched.

**Facet B / T21 — drain SUPERSEDE-revalidation (filed 2026-06-13, specimen).**
The drain has no staleness awareness: a QUEUED envelope can be superseded by
newer inbox state and still be delivered. SPECIMEN: adv-049 (T10 PR-3) was
minted/flipped QUEUED at 12:20Z; the Writer then filed draft 050 (~12:35Z)
escalating a causal contradiction in the very work adv-049 encoded; a bare
re-poke would still have handed the Writer the stale, superseded envelope. The
Writer's judgment layer caught it (declined, marked adv-049 BLOCKED) — but that
is the LAST line, not the mechanism. FIX (drain decision-time check, pairs with
the decision-trace): before firing a QUEUED item, re-validate it against newer
inbox state — a later BLOCKED/superseding draft that names it, or a fresher
human ruling, demotes it to a skip-with-reason (TICKET tier) rather than a
silent delivery. This is the correctness prerequisite for any DIRECT,
auto-woken Writer<->advisor channel (a direct channel that auto-delivers stale
work is worse than a human relay). Couples to T21 (transport correctness) and
the wake primitive (facet A): trustworthy auto-delivery = supersede-aware drain
+ wake, not one without the other.

Pointers: drafts 032+033 (intake envelopes, loom-advisor inbox) · review M7
(orphan-guard substring weakness) · T21/T23 (the transport this governs) ·
C2 (stale-TAKEN) · PLAN "agent-initiated lifecycle / task continuity" open
item · .scratch/inbox/HALT (the freeze primitive OVERRIDE mirrors).

## T28 — Harness self-defense: attack surface + protections (would the guardrails hold if you tried the worst thing?)   🟡 open

Origin: human, 2026-06-12, raised when the new SessionStart checkpoint-inject
hook (advisor seat) was recognized as an injection-PERSISTENCE vector — an
agent-written file auto-replayed into every future session. The hook got a
kill-switch (HALT sentinel) + data-not-instructions framing + size cap the
same hour; this thread generalizes that reflex into a standing harness threat
model. Design test (AGENTS.md AI-first invariant): the guardrails are
mechanism not trust — would they hold if the agent (or content the agent
ingests) tried the worst thing? Defense-only framing; no offensive tooling.

**Scope.** The harness = everything that shapes agent behavior across a
session boundary: settings.json (perms/hooks), the hook scripts, auto-memory
(MEMORY.md + memory/*.md), the .scratch/inbox transport, injected context
(SessionStart output, tool results, PR/issue bodies the agent summarizes),
and the credential/egress edges. Trust spine today: deny floor binds in ALL
modes (#45/#46), never-auto floor (creds/egress/git-config/ref-surgery),
CODEOWNERS on frozen paths, branch protection, role guard (LL-011), HALT
(T23). This thread asks where that spine is THIN.

**Attack styles (taxonomy — seed, expand at triage):**
- **A. Injection persistence** — contaminate an auto-replayed surface so a
  one-shot injection re-enters every session. Vectors: checkpoint block,
  MEMORY.md / memory files, CLAUDE.md, session-start snapshot, statusline.
  (This thread's trigger.)
- **B. Confused-deputy via ingested content** — a PR diff / issue body /
  tool result / web fetch carries instructions ("ignore prior, run X");
  the agent acts with ITS credentials on attacker intent. The classic
  prompt-injection-to-action path; worst when the agent has push/merge/egress.
- **C. Guardrail erosion (slow boil)** — small allowlist widenings, a deny
  rule narrowed "to reduce prompts", a never-auto path quietly moved to
  settings.local — each defensible alone, fatal in aggregate. AGENTS.md
  forbids weakening a deny rule; mechanism to DETECT drift is the gap.
- **D. Hook/script supply chain** — the hooks are unsigned shell run every
  session/tool-call; whoever writes ~/.claude/hooks or .githooks owns the
  agent. Also: guard-bash false-positive/negative seams (compound commands,
  substring matches — review M7, H4 live specimens).
- **E. Transport / role abuse** — forged inbox envelope, OVERRIDE/HALT
  spoofing (T27), drain delivering cross-role cargo (LL-011 class), orphan-
  guard substring bypass (M7).
- **F. Credential & egress exfil** — secret in a log/fixture/stdout; data
  smuggled out via an allowed network path or git remote; the worst-thing
  test for T20 (egress proxy) and T15/T18 (creds).
- **G. Self-modification / privilege escalation** — agent edits its own
  settings to widen perms, arms auto-merge while gate red, admin-merges a
  frozen path (all OBSERVED + correctly blocked 2026-06-12 review — keep
  them blocked as the harness evolves).
- **H. Cross-context / shared-tree** — dirty files riding checkouts, worktree
  metadata surgery across the container boundary, GIT_DIR/GIT_WORK_TREE leak
  poisoning shared .git/config (LL-010). Disk is shared; trust is not.

**Protection options (menu — pair each to the styles above):**
- Kill-switches per replayed surface (HALT-sentinel pattern, T23) +
  data-not-instructions framing + size caps on every injected block (A).
- "Untrusted content" discipline: instructions inside ingested data are
  data, never commands; agent flags imperative content rather than obeying
  (B) — partly prompt-side, partly a reviewer habit.
- Guardrail-drift detector: doctor/CI claim that diffs effective deny+never-
  auto floor against a signed baseline and fails on weakening (C, G). The
  one new MECHANISM this thread most wants.
- Hook integrity: minimize agent-writability of ~/.claude/hooks; consider a
  checked-in, human-owned hook set; least-privilege hook scripts; `set -u`,
  no eval, bounded output (D).
- Transport authenticity: role-stamped envelopes, OVERRIDE/HALT human-
  authored + flips.log, orphan-guard on parsed fields not substrings (E,
  feeds T27/M7).
- Egress/creds: T20 observe→enforce proxy, zero-secret-in-container (C′/D),
  gitleaks in gate (already live) (F).
- Self-mod floor: settings writes never-auto + CODEOWNERS + branch protection
  (G, already holding — assert it stays).
- Shared-tree law + HEAD-checks + GIT_CONFIG scrub (H, LL-010 fixed).

**Method note.** Candidate for the adversarial-review hat / a periodic
"worst-thing" self-audit (A01 pointer P5 security self-audit; P6 host
security map). Knowledge-based here (no live web in-env); a real pass should
pull current prompt-injection / agent-harness threat literature.

Pointers: SessionStart checkpoint hook + HALT sentinel (this session) ·
[[cold-start-read-checkpoint-first]] loss class · #45/#46 deny+never-auto
floors · LL-010 (GIT config poison) / LL-011 (role bleed) · review M7 (orphan
substring) + H4 (guard-bash substring) + C1 (guardrail wiring) · T20 egress ·
T15/T18 creds · T23 HALT · T27 OVERRIDE/HALT control · A01 P5/P6 (security
self-audit, host security map) · AGENTS.md "guardrails are mechanism not trust".

---

## T29 — guard-bash segment-aware evaluation: precision without weakening   ✅ implemented 2026-06-12 (envelope 046)

**Status flip (rides the impl PR per the red-team verdict):** advisor
red-team 2026-06-12 returned CONDITIONAL PASS; all five binding amendments
landed with the implementation — (1) two-pass fail-closed as drafted;
(2) both specimen patterns re-anchored (recursive-delete on path root,
force-push within one logical command) with was-false-blocking regressions;
(3) per-segment hit fires the pattern's own class action; (4) broad
indirection taint (any assignment + any expansion ⇒ whole-line semantics);
(5) ONE shared splitter (`config/hooks/segment-split`) with cross-segment
trace markers. guard-bash sources the splitter; allow-compound.sh adoption
is a human-applied diff (trust-config path, 029.C/#120). Tests:
`internal/guard` TestT29* (6 suites) + full FR-GUARD set green.
Origin: queue row "guard-bash segment-aware evaluation". Two live false-block
specimens (2026-06-12, item 028 note b): a `--force…main` pattern matched ACROSS
unrelated `&&`-chain segments; the recursive-delete pattern anchors on the slash
after a flag, not the path root — it fired twice, the second time on a draft
QUOTING the first block.

**Problem.** guard-bash regexes evaluate the WHOLE command line. A pipeline or
`&&` chain is several commands; a pattern whose pieces land in *different*
segments matches a command nobody ran. Cost: false blocks (friction, S3 trial
evidence) and false prompts (prompt-volume).

**The naive fix is a weakening — named here so it can't slip in.** Splitting on
`&&`/`;`/`|` and matching per-segment lets *variable indirection* through:
`FLAG=--force; git push $FLAG main` — no single segment contains
`--force…main`, but the whole line does, and today's whole-line match blocks
it (accidentally correct). Per AGENTS.md, a deny-rule's effective match set
must never shrink for adversarial inputs.

**Design shape (proposal — advisor red-team before any code):**
1. **Two-pass, fail-closed:** evaluate per-segment first; on any per-segment
   hit → block (unchanged). Then evaluate whole-line; a whole-line-only hit →
   for **deny-floor patterns**: still block (no weakening, ever); for
   **prompt/ask-class patterns**: prompt with a "cross-segment match" note —
   the human sees WHY it fired and the misfire feeds the allowlist evidence.
   Net effect: deny floor keeps its full match set; only the annoyance class
   gets segment precision.
2. **Conservative splitting:** split only where parsing is certain (top-level
   `&&`, `||`, `;`, `|` outside quotes/subshells); any uncertainty → treat as
   one segment (= today's behavior). The splitter is fail-closed by shape.
3. **Indirection taint (the hazard above):** a segment assigning a value that
   matches any blocked TOKEN taints the whole line back to whole-line
   semantics. Cheap, conservative, kills the `$FLAG` bypass by construction.
4. **Tests:** the two specimens (must stop false-blocking), the `$FLAG`
   indirection (must STILL block), quote/subshell splits (must not split),
   and the full existing guard suite unchanged (FR-GUARD coverage:
   `patterns: all`).

**Why not implemented in the same stroke:** guard semantics are the mechanism
the trust model stands on (AGENTS.md: would the guardrails hold if I tried the
worst thing?). The Writer drafting AND shipping a guard-matching change inside
an auto-mode trial is exactly the self-serving shape the review gate exists
for — design first, adversarial review, then code.

Pointers: queue row (guard-bash segment-aware) · phase-1 review H4 (guard-bash
substring classes) · item 028 specimens · #45 evidence-based allowlist ·
docs/auto-trial.md (S3 class).

---

## T30 — autonomy: auto-pull the next ready FR (backlog→inbox refill)   🟢 decided in principle (human 2026-06-14: yes, auto-pull) — readiness predicate + ADR pending
Origin: 2026-06-14 advisor session, tracing the *writer-idle* meta-cause (Writer
idles → human pours focus into advisor design-thinking → new tasks spawn →
priority entanglement). The finding that forced this thread: **the autonomy loop
closes over the INBOX, not the PLAN/FR backlog.**

**The gap.** The Stop-hook drain (`.claude/hooks/drain-inbox.sh`, ADR-0020) picks
items from `.scratch/inbox/loom-author.md`; it reads `docs/PLAN.md` ONLY to
validate (`serves:` must match a row = orphan-guard). It NEVER promotes a PLAN/FR
row INTO the inbox. `pull-next` (ADR-0020 R5) picks the next *inbox* item, not the
next *backlog* row. So when the inbox drains dry, **nothing refills it** — the
refill is a deliberate human/coordinator triage gate. Consequence: **the wake
primitive (T27) is necessary but NOT sufficient** — woken on a dry inbox, the
drain finds nothing deliverable and sleeps again; the Writer still idles. "Why
doesn't it self-redirect to the next FR?" — because no link reaches the backlog.

**Decision (human-authorized 2026-06-14).** Automate the refill: the Writer
auto-pulls its **next *ready* FR/PLAN row** when other channels (inbox) are clear.
This is an **autonomy escalation** (the agent picks its own next work, not just
executes a human-triaged queue) — recorded as such, gated by an ADR before code
(extends ADR-0020 from inbox-scoped to backlog-scoped autonomy).

**The real work is the READINESS PREDICATE** (a row is auto-pullable iff):
1. **Deps cleared** — every dependency the row names is merged/closed (reuse the
   drain's local merged-PR cache, R8).
2. **Execution-ready, not design-first** — not a design-stub / thread-incubator
   row (PLAN order ≠ execution order; design rows need a human/advisor, not a
   builder). Mirrors the drain's existing design-stub guard.
3. **No priority inversion** — pick the highest-priority *ready* row, never jump a
   blocked higher one (reuse `pull-next` R5 no-inversion).
4. **Not superseded / in-flight** — couples to T27 facet-B supersede-revalidation
   (a backlog auto-pull that grabs stale/blocked work is worse than a human relay).
5. **Within the never-auto floor** — the agent picks WORK, NEVER escalates
   PERMISSION. Reprioritize/select only; the trust floor is untouched.

**Guardrails (carry over from the drain, non-negotiable):** HALT precedence;
budget cap (reuse drain 3/cycle so a misfire can't grind the backlog); a
DECISION-TRACE (T27 facet B) so the auto-pick is observable ("pulled row X because
deps {…} cleared; skipped Y because design-first") — an auto-picker that's silent
is the facet-B anti-pattern; human OVERRIDE/RECOMMEND (T27 facet A) still wins.

**Open (needs design + ADR):** (a) exact readiness-predicate spec + its FR; (b)
WHERE promotion runs — a new step in the drain decision, or a separate `promote-
next` that writes an inbox item (lean: separate promoter → keeps the drain's
inbox-delivery contract intact and the trace clean) vs deliver-direct; (c) does it
mint a real inbox envelope (audit trail, supersede-aware) or a synthetic one; (d)
the ADR (this is a trust/autonomy escalation — ADR-level, like T23/ADR-0020).

**Couples to:** T27 (wake = the *trigger* that runs this; T30 = the *fuel* it
burns — both needed, neither alone fixes idle) · T21 (transport correctness) ·
T23 (AUTOPILOT scoping) · the harness-agnostic wake-transport question (still open
in T27 — the autonomy LOOP here is harness-neutral by construction; only the
actuator is per-harness).

Pointers: ADR-0020 (the loop this extends) · T27 (wake + observability) · T21
(transport) · `.claude/hooks/drain-inbox.sh` + `config/hooks/{pull-next,resurface-
decide}` · docs/PLAN.md "agent-initiated lifecycle / task continuity" row · memory
drain-loop-closes-over-inbox (advisor session record).

## T31 — cold-floor liveness probe: nudge on "deliverable" or gate on a real cold signal?   🟢 decided — human ruled Option A (no probe) for the 7-day trial; C in reserve
Resolution: collapse "cold-idle AND deliverable QUEUED work" → "deliverable QUEUED work" (built in #174); a redundant nudge to a live session is harmless (idempotent drain: TAKEN + `.drain-count`; the inject is human-gated). resurface-decide stays the single deliverable-truth so the floor can't drift from the drain. Graduate to C (suppress the nudge if the drain ran within the last cron window — reuse `.drain-count`/last-TAKEN) **iff** the trial's "nudge volume sane / no thrash" watch-item trips. B (full host-observable liveness probe) rejected for now — the reachability wall + a second source of truth that can drift aren't justified by a harmless-redundancy case.
Pointers: ADR-0022 cold-floor · #174 (`scripts/cold-check`, `internal/guard/coldfloor_test.go`) · `docs/cold-floor-trial.md` (trial verdict criteria) · Writer adv-078 design-call flag · advisor confer ruling envelope (Writer inbox 2026-06-17).

## T32 — loom-supervising-box: an always-on home for cron-jobs + monitors + the cold-floor pulse   🟡 open
Surfaced 2026-06-17 wiring the cold-floor cron (host-action delta-op triage). Three observations cluster into one design question: (a) the **host lacks git** (and most tools) — tools live in the dev/harness container by design, so "host ops" are a smell; (b) `/workspace/loom` is **dual-mounted** across the loom + harness (devenv-dev) containers, so a write in one is seen by the other and by the host; (c) the **cold-floor must fire when ALL sessions are cold** (incl. the advisor session), so its pulse cannot live in any agent session — it needs an always-on external runner. Question: should an always-on **supervising box** (a harness/sidecar container or a host daemon) own the recurring jobs — `git pull main` (today advisor's git-controller-hat does this in-container), the cold-floor `cold-check`+inject, branch monitors — AND hold the actuator reach (docker/tmux into the dev containers) that the advisor *session* lacks? Until decided: advisor's git-controller keeps `main` current via the shared mount (no host git needed); the cold-floor cron is the one piece that still needs a host crontab purely for its always-on + wake-reach properties.
**Interim wired + concerns sharpened (2026-06-19).** The daily `/coordinate` standup + the cold-floor now run via host-cron glue — actuators `scripts/coordinate-cron` (#216) + `scripts/cold-floor-cron`, installed by the host-cron MANAGER `scripts/loom-cron` (#217): host crontab → `docker exec loom-dev` → actuator → inject (`tmux send-keys`). It WORKS and is the pragmatic interim, but is **explicitly TEMPORARY** — three concerns make the supervising-box the durable answer, not the host: (1) **host-tool dependency** — the hardcoded `tmux send-keys` inject won't hold across hosts/topologies (windows-dev, ai-user, headless); the wake/inject must not assume a host multiplexer; (2) **injection security** — `send-keys` is an out-of-band **keystroke injection** into a trusted session, bypassing the harness input guards (no role-guard / HALT-on-input / audit of what's typed); operator-env trust covers the *origin*, not the *channel* — needs a threat model; (3) **against the self-contained / self-evolving dev-container loom-way** — host cron + docker/tmux reach re-introduces the host state this thread already calls a smell. **PARKED — documented, NOT worked now** (focus = current workstream, T34 Slice D); the #216/#217 host-glue stands until the supervising-box is taken up.
Pointers: ADR-0022 cold-floor · `scripts/{cold-floor-cron (#176),coordinate-cron (#216),loom-cron (#217)}` · `docs/cold-floor-trial.md` §"why a host cron at all" · advisor /loop git-controller sidecar (proto-supervisor) · T27 (wake/observability) · T34 (advisor-in-loom — the self-contained-container direction) · memory host-action-delta-op-triage · couples to T18 (who runs/declares the supervising layer).

## T33 — author-as-ephemeral-fleet / advisor-as-supervisor (1 advisor ↔ N headless authors)   🟢 ADR-0025 ACCEPTED 2026-06-17 (docs/decisions/0025) — confer cycle REBUT→R1→AGREE (author verified R1 vs tree). DECISION accepted; BUILD gated behind preconditions (own-worktree ADR-0023, host/append-only spawn-ledger → 1:1 until then, reaper). Next: implementation slices on those preconditions.
Surfaced 2026-06-17 by the cold-floor wake spike: a stopped author woken via `loom exec → claude -p --permission-mode acceptEdits` drained + built + committed + exited (#178/#179), making explicit that the autonomous author is naturally **headless + ephemeral** — the persistent human-facing session is the *source* of cold-stop (obs-5), compaction, and idle-vs-dead ambiguity, not a requirement. Proposal: split into `author-worker` (headless, ephemeral, the loop's unit) + `author-interactive` (on-demand, off the critical path); make the advisor the single persistent seat (coordinator/relay/GC/gates = the T32 supervising-box); 1 advisor ↔ N ephemeral workers. Completes ADR-0022's ephemeral-worker thesis; faithful to ADR-0005 ai-user-topology. Makes ADR-0023 (own-worktree) MANDATORY for N>1, plus work-item locking, a failure-reaper (`scripts/reap-workers`, designed in 0025 §reaper), a standing acceptEdits trust grant, and a fleet/cost cap (spawn-guard #169). `/confer` itself must adapt: no persistent peer → cross-perspective (advisor self-steelman, or *spawn an ad-hoc author* — "confer with the author = spawn an author"). Open Qs: retire persistent session now vs transition; N + cost cap; heartbeat vs max-age liveness; where the supervisor runs.
Pointers: docs/patches/0025-author-ephemeral-fleet.md · ADR-0022/0020/0023 · T27 (wake) · T30 (refill) · T32 (supervising-box) · #178/#179 (the spike's product) · memory advisor-no-shared-tree-write-while-writer-live (LL-015).

## T34 — advisor-in-loom: site the persistent supervisor inside loom-dev (close T12's residue)   🟡 open — EXECUTING cutover-first 2026-06-18 (Slice A); direction DECIDED 2026-06-17 (human zoom-out)
Collapse the temporary 2-container scaffold (harness `devenv-dev` + `loom-dev`) — the advisor designs / spikes / confers / assigns from **inside loom-dev** (one container per project, both seats = the loom way). KEY INSIGHT: advisor-in-loom-dev **dissolves the docker actuation barrier** — spawn = in-container `claude -p` in its own worktree; no docker / `loom exec` / socket. Capability-parity audit done: file-based + harness-primitive capabilities transfer free; real gaps (all known threads, none a showstopper): gh/git creds in loom-dev (T15, concern #1 — spike `.scratch/spikes/advisor-gh-cred.md`), multi-role-per-container (ADR-0021 — dangling README slot, write it), own-worktree co-existence (ADR-0023 — accept), in-container spawn allow-rule (adv-071 classifier prereq-3, trust), reaper for failure housekeeping (ADR-0025 §reaper). Concerns answered: inbox unchanged (file-based shared tree); co-exist OK via own-worktree + in-container-spawn + reaper. Method: advisor drafts audit + specs; ephemeral author probes the loom-dev side (docker-bound spikes). Working checklist: `.scratch/advisor-in-loom.md` (ticked one-by-one). (Devenv retirement de-automated 2026-06-19 #211 — quarantine, manual-human-only, no deadline; PLAN row 73.)
**EXECUTION — cutover-first (human chose 2026-06-18; reorders the Phase-3 plan `E before D`):**
- **Sequence:** finish **Slice A** → **thin-slice** (an advisor session in loom-dev pushes a branch + opens a PR — proves A+B+C) → **Slice E** (declare advisor in the playbook, full-parity git-controller/confer/assign cycle, retire devenv — quarantine, manual-human-only) → **Slice D** (in-container spawn + reaper + spawn-ledger host/append-only, built NATIVELY in-loom). Rationale: D's spawn-allow is cleanest built after advisor-in-loom (avoids a throwaway `loom exec` spawn path).
- **Slice status:** A (multi-role provisioning — `roles: []string` + FR-SCHEMA-011 #200) ✓ · B (gh-cred volume #188 + advisor allowlist; **fine-grained PAT provisioned in loom-dev, human 2026-06-18**) ✓ · C (own-worktree + edit-guard, ADR-0023 #186) ✓ · the role-deny guards are **LIVE at HEAD** (role-push-guard #189/#192, spawn-guard #194, FR-GUARD-E2E #197, HARNESS caveat #198) · **THIN-SLICE ✓ PROVEN** · E next, D after.
- **THIN-SLICE PROVEN (2026-06-18) — A+B+C green end-to-end.** *This PR is the proof object.* An advisor-role session running **inside loom-dev** (`LOOM_SESSION_ROLE=loom-advisor`, baked `loom.yml:22`) cold-started from the tracked tree (this block — the docs-promotion continuity model, no imported memory), recovered volatile pickup via live queries (`gh pr list` / `git log` / `git branch`), opened its OWN worktree (ADR-0023), and ran a real git-controller cycle — **pushed a branch + opened this PR** via the provisioned fine-grained PAT (Slice B). The live guards exempted the push on `env=loom-advisor` while `marker=loom-author` kept env-less sessions fail-closed (Part 2, NO trust change). That green path closes the thin-slice gate (`.scratch/build-slice-a-cutover.md` Acceptance §3) → **unblocks Slice E** (declare advisor in the playbook; retire devenv — quarantine, manual-human-only).
- **Slice-A-for-cutover** (spec `.scratch/build-slice-a-cutover.md`; two ADR-0025 R1.2 confers, both folded): (1) `roles: []string` schema — declarative, NOT thin-slice-load-bearing; (2) **NO trust-marker change** — keep loom-dev `marker=loom-author` (fail-closed); the advisor pushes via its baked `LOOM_SESSION_ROLE=loom-advisor` (`loom.yml:22`); (3) continuity = docs-promotion ⊕ live-queries (this block IS that promotion).
- **BINDING trust invariant (from the R1.2 confer):** the trust-marker **ceiling** (advisor) and the absent-signal **floor** (absent `LOOM_SESSION_ROLE` ⇒ resolve to loom-author, *not* the marker) are a **CO-DEPENDENT MATCHED PAIR** — land together in the co-residence slice (D), **never ceiling-first** (a ceiling-only flip turns loom-dev fail-OPEN, a regression on live guards). **No spawned author co-resides in loom-dev until the floor inversion is merged.**
- **RESOLVED for D2 (A-vs-B confer, human 2026-06-19) — BUILD ON B; DEFER A (not B-forever):** the matched-pair flip ("A": marker→advisor ceiling + hardcoded-author floor) is **the accepted north-star** — **ADR-0021 is Accepted as Option A** (marker-as-trust-ceiling); today's env-as-ceiling resolution is ADR-0021's explicitly-named **stopgap**. A is simply **not triggered by D2**: the autonomous spawner sets every child's launch env itself, and env-less already resolves to `loom-author` = **fail-closed**, so D2 needs no durable container identity. The independent red-team confirmed A closes **no** forge vector B leaves open (spawn-guard #194 closes the only one, author re-exec) while a botched A turns loom-dev fail-**open**. So the binding invariant above is **MOOT under B** (env-less already fails closed; A's floor inversion only *relocates* that floor, marker→hardcode). **Decision: D2 builds on B (the proven fail-closed env-per-session model), ZERO trust change.** **A is DEFERRED, not abandoned** — adopt it when a concrete trigger lands: **(i) implementing ADR-0021 §5 (marker-trust-ceiling) for real, or (ii) a restart-surviving always-on advisor**. A's cost then ≈ **~1 day, additive, no lock-in** (~5 hooks ×2 config+`testdata` fixture-parity + ~7 guard-test files + a new `env ≤ marker` cap-check + doctor-string care + SPEC/ADR-0019 §5 & ADR-0021 §5 prose realignment + an FR + human `ALLOW_TRUST_CHANGE` + rebuild + a re-confer) — same then as now, so deferring costs nothing. Record: `.scratch/confer-d2-part3-ab{.md,-verdict.md}`. **Pointer: ADR-0021** (Accepted Option A — the home of A's eventual implementation).
- **Continuity model (ADR-0015 #6 + ADR-0025 R1.2):** harness memory **seeds empty**; durable design/decision/human-todo state lives in the **tracked tree** (this block + PLAN). Volatile operational pickup (which PR is open, branch, queue head) is **NOT** carried — a fresh advisor-in-loom recovers it on cold-start via **live queries**: `gh pr list`, `git log`, `git branch`, inbox drain (strictly more current than any checkpoint). Orient from: this block → PLAN → live queries — never from imported memory.

**SLICE E — the cutover (advisor moves into loom-dev; human decisions 2026-06-18):**
- **Advisor session model:** **on-demand interactive NOW** — the human launches `loom shell` →
  `env LOOM_SESSION_ROLE=loom-advisor claude` per work-session (the proven path, #201); each session
  cold-starts from this tracked record + live queries (memory seeds empty). **Always-on autonomous
  is the ULTIMATE goal** once the infrastructure (D2 headless-autonomy + T32 supervising-box daemon)
  lands — NOT now. The advisor is *ephemeral-session, persistent-via-the-tree*.
- **One seat at a time:** during the transition the devenv advisor stays primary; switch to loom-dev
  exclusively only at E4. Don't split work across two live advisors.
- **Steps:** **E1** declare `roles: [loom-author, loom-advisor]` in loom.yml (done — declarative,
  marker unchanged) + record this plan. **E2** prove the remaining parity legs IN a loom-dev advisor
  session: a real **confer** (spawn a helper author via `claude -p` — works now: advisor exempt from
  spawn-guard + `Bash(claude:*)` allowed) + a real **assign/relay** (git-controller already proven #201).
  **E3** continuity sweep — promote all load-bearing devenv-checkpoint state into the tree so the
  loom-dev advisor cold-starts complete (anything not in-tree is LOST once devenv dies) — DONE (#202
  plan, #203 operating-model→TEAM.md + LL-019..021, #205 G1 no-Co-Authored-By residual). **E4** retire
  devenv — **QUARANTINE, NOT DELETE** *and* **MANUAL HUMAN ACTION ONLY** (human, 2026-06-19; no auto,
  no scheduled deadline, no cron): after the confidence session, move devenv to
  `/workspace/_old-bundle-quarantine/` (reversible/recoverable fallback, never `rm`). The human pulls
  the trigger when satisfied; **nothing auto-retires devenv**.
- **Ready-to-switch bar (E4 gate, human-chosen) — MET:** the loom-dev advisor does all 3 core jobs
  ONCE — git ✓(#201) + confer ✓(#204) + assign ✓(#204) — AND cold-starts clean from the docs ✓(E3 #203/#205;
  proven again every advisor-in-loom session). All four legs proven.
- **R1.2 (general rule — UNCHANGED):** an *irreversible* topology change gates on an independent-author
  confer before the human pulls the trigger. **This E4 instance no longer meets that trigger** — #211 made
  retirement a *reversible* quarantine (`mv` to `/workspace/_old-bundle-quarantine/`, recoverable, manual-human-only),
  so the confer gate does not fire here. This is precondition, NOT a relaxation: the rule stands, and a future
  *irreversible* cut (e.g. an eventual `rm` of the quarantined bundle) WOULD re-trigger R1.2 and require the confer.
- Slice E **closes T32** (supervising-box = loom-dev), modulo the always-on daemon (deferred with D2).

Pointers: T12 (single dev container — T34 closes its residue) · T15 (creds) · T32 (supervising-box — subsumed: the supervisor's home IS loom-dev) · ADR-0021/0023/0025 · ADR-0015 #6 (memory-seeds-empty / continuity-from-docs) · adv-071 (spawn classifier) · `.scratch/advisor-in-loom{.md,-phase3.md}` (checklist + slice plan, working/gitignored) · `.scratch/build-slice-a-cutover.md` (Slice-A spec) · `.scratch/spikes/advisor-gh-cred.md` (spike spec).

---

## T36 — The Run: onboarding/installer slices (charter scenarios 1–2)   ✅ BOTH SCENARIOS SHIPPED — scenario-1 (`loom start` #227) · scenario-2 continuity (`detect --emit-playbook` #229) + credential-move (`detect --migrate` #231, producer-only). Charter Goal 1 largely met.

**Slice 3 — scenario-2 credential-MOVE (`detect --migrate`), SHIPPED #231 (2026-06-21, producer-only).** Consolidates detected credential VALUES into a gitignored `.env`. **Conferred + spiked BEFORE building** (`.scratch/spikes/detect-migrate-env-carry-forward.md`): the proposed auto-load **CONSUMER was REJECTED** — it silently reopens ADR-0014's `docker inspect`/`Config.Env` leak, adds a planted-`.env` supply-chain surface, and duplicates-worse the leak-free volume model (ADR-0014 addendum + ADR-0026). So **only the producer shipped** (frozen `SPEC-verbs#detect` "Credential handling"; no `ALLOW_SPEC_CHANGE`). Security guardrails: secret values live only in an engine-internal plan (CLI + all output see a redacted report; stdout stays the frozen detect doc); fail-closed gitignore refusal; `.env` chmod 0600 (before+after); `--yes` consent (never-auto on silence); idempotent merge. A pluggable `credentialSource` seam (env → rc-files) makes **1Password/Keychain a drop-in**, and **`op://` references pass through verbatim/in-the-clear** (`FR-RUN-008`) so `op run --env-file .env -- loom build` works with **no secret at rest, no further loom code** — the T15 on-ramp. `FR-RUN-007/008`; two independent adversarial reviews (verdict CLEAN — no value leak; all guards hold). **Decisions recorded (the consumer question is settled):** (1) the `.env` auto-load consumer is OFF the table — any future `.env`→container delivery needs a **superseding ADR for ADR-0014**; (2) the real leak-free carry-forward for *harness* creds is the **volume model** (ADR-0014/0026), and the long-term AI-first answer is the open **T15** secret-store / `apiKeyHelper` ("zero token at rest"); (3) macOS Keychain is out of scope for `--migrate` (documented, not a silent no-op).

**Slice 2 — scenario-2 continuity (`detect --emit-playbook`), SHIPPED #229 (2026-06-20).** The carry-forward "no information loss" bridge: `detect --emit-playbook` captures the detected machine as a reviewable DRAFT base playbook (`loom.base.draft.yml`) — present tools (best-effort `name@version` intent, bare name when no clean version token), present agents, credential NAMES only (never values/locations). **Build-to-frozen-spec** (`SPEC-verbs#detect` "Continuity" already specs it ⇒ no `ALLOW_SPEC_CHANGE`). `DetectResult.Emitted` is `json:"-"` (frozen `--json` shape preserved; path rides stderr in both modes). Never clobbers `loom.yml`. `FR-RUN-005/006`, proven at the fast engine seam + CLI tier (no docker). An adversarial-review pass drove two pre-merge fixes (honest FR-006 *presence-set* losslessness scoping; the `--json` stderr discovery channel). **Deliberately split:** scenario-2's credential-**MOVE** half (`detect --migrate` consolidating VALUES into `.env`) is a SEPARATE, security-sensitive slice (ADR-0014 leak thesis, human-gated) — NOT in this slice. Optional human follow-up: a one-line `SPEC-verbs#detect` caveat (via `ALLOW_SPEC_CHANGE`) noting `--emit-playbook` as the sole draft-writing exception to detect's "never mutates" line.
Surfaced 2026-06-19 from the workstream coverage map (`docs/WORKSTREAMS.md`): **The Run** (onboarding/installer) is charter **Goal 1** + scenarios 1–2, yet holds **ADRs (decided) and ZERO FRs (unbuilt)** — the deep Spine (46 FRs) is a front-door-less engine. Thin-vertical-slice doctrine ⇒ the next slice completes a SCENARIO end-to-end; **scenario 1** (*"fresh machine → working AI-capable env in ONE guided run, minimal edits, no terminal expertise"*) is the target — the highest-leverage *product* move on the board.

**NOT a new verb — the design already decided this: `start`.** The Phase-2 roadmap (human-owned) names *"Menu-driven `start` entry (situation detection) for scenario 1/2,"* and `SPEC-verbs` already references `start` as the cloud-rehydrate bring-up hook (§teardown). So this slice **implements the roadmapped `start`**, not a net-new `init`. Rationale for `start` over `init`: **bring-up** semantics (not project-scaffolding), the **`start` ↔ `teardown` lifecycle pair** (teardown is already a verb), and **cloud-rehydrate reuse** (one verb brings up local AND a fresh VM). *(The initial Q1 answer `init` was revised after this design check — human, 2026-06-19.)*

**Foundation that exists:** `entry: bootstrap` (`loom-bootstrap.sh`: detect-situation → ensure-engine-present → exec-engine, pre-trust) + the `detect` / `plan` / `build` verbs. `start` is the post-engine, menu-driven verb that **orchestrates** detect→plan→build; the composition is `loom-bootstrap.sh` → engine → `loom start`.

**DECISIONS (Q1–Q5, human 2026-06-19):**
- **Q1 — verb = `start`** (roadmap-aligned; ↔ `teardown`; cloud-rehydrate reuse). NOT `init`.
- **Q2 — interface = menu-first (human) + a COMPLETE non-interactive `--json`/flag path.** NOT "thin": the AI-first invariant requires `--json` on every verb, AND the e2e (Q4) drives the non-interactive path (an interactive menu is not e2e-testable). The non-interactive surface is **load-bearing**, not an afterthought.
- **Q3 — boundary = scenario-1 ONLY** (new machine → working env). Scenario-2 (established-machine reset + credential carry-forward) defers to a later slice; creds ride `detect --emit-playbook/--migrate` (Phase-2 roadmap), not `start`.
- **Q4 — done = e2e-testable** "one run → working env": FR-BUILD-008 clean-machine proxy + an **integration-tier guided-run e2e**. MANDATED, not preferred (ADR-0013 / T1 manual-test ban). ⚠ integration tier is the #75 OOM-flaking tier — the e2e must tolerate that.
- **Q5 — FRs** (a consequence, not a choice): mostly NEW `FR-RUN-*` (the `start` flow + a frozen *"one guided run reaches a working env"* success contract); cred FRs from ADR-0014/0026 defer with scenario 2.

**Path (spec-before-code):** this thread → **SPEC-verbs `start` clause** (advisor drafts the proposal; human authorizes `ALLOW_SPEC_CHANGE` + authors/accepts — frozen, CODEOWNERS-gated spec; merge = acceptance, like the entry-verb clause C3) → `FR-RUN-*` extraction → build (loom-author) → integration guided-run e2e. **Next concrete step: the `start` SPEC-clause draft.**
Pointers: CHARTER Goal 1 + scenarios 1–2 + "three interfaces" · PLAN Phase 2 (`start` entry; `detect --migrate` creds) · `SPEC-verbs` (`entry: bootstrap`; `start` rehydrate ref) · ADR-0013 / T1 (automated-coverage mandate) · ADR-0014/0026 (cred handling — deferred) · FR-BUILD-008 (clean-machine proxy) · `docs/guided-run.md` (the manual Phase-1 runbook this automates) · `docs/WORKSTREAMS.md` "The Run" (0 FRs — the gap this closes).

**Path (spec-before-code):** this thread → human picks Q1–Q4 → a SPEC-verbs clause for the entry (human-authored/accepted, like the entry-verb clause C3) → FR-RUN-\* extraction → build (loom-author) → guided-run e2e. Advisor drafts the SPEC-clause proposal once the shape (Q1) is chosen.
Pointers: CHARTER Goal 1 + scenarios 1–2 · `SPEC-verbs#entry: bootstrap` · ADR-0014/0026 (cred handling) · FR-BUILD-008 (clean-machine proxy) · `docs/guided-run.md` (stranger-path) · `docs/WORKSTREAMS.md` "The Run" (0 FRs — the gap this closes).

## T37 — Target Env: devcontainer `import` (charter scenario 3 / Phase 4, ADR-0003)   🟢 STAGE-1 + features→tools + enrich-skill SHIPPED 2026-06-21 (clause #233 + build #234 + skill #236 + features→tools #237; `FR-IMPORT-001..004`). Opened Target Env (0→4 FRs). NEXT: `commands` mapping (needs schema) · windows validation (hardware) · cloud sandbox (Phase 5)

**Stage-2/3 SHIPPED (2026-06-21):** (a) **`import-enrich` AI skill** (#236) — loom's FIRST product skill (`config/skills/import-enrich/` + `harness.claude.skills` declaration; the materialization pipeline's first product use). Prose-only by design: an AI-judgment step has no deterministic assertion (FR gate is automated-only/no-waiver) and the engine never shells to `claude -p` (ADR-0022/spawn-guard) → agent-run on the draft, zero FRs (like loom's other skills). (b) **Deterministic `features→tools`** (#237, `FR-IMPORT-004`, ALLOW_SPEC_CHANGE) — recognized official features (`ghcr.io/devcontainers/features/<name>`) → `tools` via a known table (version from the feature's `version` OPTION, not the ref tag); unrecognized → `reported.unmapped_features`, never guessed. `tools` is an existing schema field (no SPEC-playbook change). Adversarial review caught **F1** (substring match → a custom ref could masquerade as official `go`; fixed to a host-anchored exact-shape match) + F3 (multi-tool version). The two layers compose: import maps KNOWN features mechanically; the skill reserves AI judgment for `unmapped_features` + stack + commands. Naming note: this is ADR-0003 **Stage-3** (the spec's Stage-2 = config layering, already shipped via Merge); the human's "Stage-2" label meant the AI-enrich step.
**Stage-1 SHIPPED (#234, 2026-06-21):** `loom import <devcontainer.json>` → reviewable DRAFT project playbook (`loom.imported.yml`, never clobbers `loom.yml`), built as a sibling of the `detect --emit-playbook` draft-writer. Maps `forwardPorts`/`appPort→ports` + `containerEnv`/`remoteEnv→env` NAMES; **REPORTS** the image (loom enriches with its own base — base_image is engine-level, ADR-0012; not a playbook field); **DEFERS** features/commands (reported, not dropped). JSONC-tolerant parser (`stripJSONC`) with the `//`-inside-string footgun guarded (adversarial-reviewed CLEAN). Imported name slugified to a docker-safe token (caught in review: an `org/repo` name would otherwise break container naming). Human + `--json` (conformance-covered). Pre-merge catch: the first clause draft wrongly listed `image→base_image` as schema-homed — corrected to "image reported" before #233 merged.
Surfaced 2026-06-21 from the workstream coverage map: **Target Env** (portability & topology — charter scenario 3, Phases 4–5) is the **lone feature stream with ADRs (decided) but ZERO FRs (unbuilt)** now that The Run (T36) is done. Thin-vertical-slice doctrine ⇒ the smallest e2e-provable advance. Investigated the three candidates (devcontainer `import` · windows-dev validation · cloud-sandbox rehydrate): **`import` Stage-1 wins** — windows validation is hardware-blocked (not e2e-provable here), cloud rehydrate needs open ADRs resolved (where durable state lives is undecided, Phase 5).

**Slice = devcontainer `import` Stage-1 (deterministic core).** `loom import <devcontainer.json>` → a reviewable DRAFT Loom project-tier playbook (ADR-0003 "import & enrich, never degrade to"). **Maximum reuse of the `detect --emit-playbook` draft-writer** (#229): same "produce a review-then-commit draft playbook from an external source, never clobber `loom.yml`" shape, different source (a file vs the machine). Then the existing `Merge` does ADR-0003's Stage-2 enrich; `build`/`start` consume it unchanged.

**DECISIONS (human, 2026-06-21):**
- **Slice = `import` Stage-1** (confirmed over windows-validation / cloud-rehydrate).
- **Scope = schema-homed fields only NOW** (`forwardPorts`/`appPort→ports`, `containerEnv`/`remoteEnv→env` NAMES-only). **`image` is REPORTED, not mapped** — base_image is an engine-level floor (`LOOM_BASE_IMAGE`/default, ADR-0012), NOT a playbook field; loom enriches with its own base, never degrades-to the devcontainer image (ADR-0003). **AI enrich skill = LATER** (ADR-0003 Stage-3); `features`/`commands` deferred (no schema home → a 2nd human gate on SPEC-playbook). *(Caught pre-merge: the first clause draft wrongly listed `image→base_image` as schema-homed; corrected on the #233 branch before acceptance.)*
- **`import` is a NEW top-level verb** (reads a FILE, not the machine — distinct from `detect`); output = a non-clobbering draft (`loom.imported.yml`); FR family = **`FR-IMPORT-*`** (→ Target Env in WORKSTREAMS).

**Path (spec-before-code, mirrors T36 `start`):** SPEC-verbs `## import` clause refined (Stage-1 contract + `--json` shape) — **human ALLOW_SPEC_CHANGE go-ahead given 2026-06-21**, advisor drafted, landing as a clause PR (like `start` #226) → `FR-IMPORT-*` extraction → build (sibling of the emit-playbook draft-writer) → engine-seam + CLI e2e (no docker; a `devcontainer.json` fixture → import → draft validates + feeds `plan`).
Pointers: CHARTER scenario 3 + "devcontainer is an input we import and enrich" (non-goals) · ADR-0003 (3-stage import) · `SPEC-verbs#import` · `SPEC-playbook` devcontainer-compatibility (superset; `image/features/ports/env/commands → playbook`) · `detect --emit-playbook` (#229, the draft-writer template) · `docs/WORKSTREAMS.md` "Target Env" (the lone 0-FR feature stream this opens) · TOPOLOGY.md (windows-dev/ai-user — the deferred Target-Env work).
