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
- **Q2.3 (yes)** "the unit gate is hermetic" becomes an invariant FR (`FR-INV-*`),
  enforced by the registry.

**Not overkill (answered):** Q2.1 + Q2.2 collapse into *one* change — the gate runs
unit tests in a scrubbed env, everywhere — which directly kills the LL-006/008
class that hit 3× this session. The thing that *would* be overkill (the AST lint,
Q2.2 option b) is excluded. Impl note: "scrub" is targeted (unset vars + hide
docker), **not** `env -i` — the gate needs go/gofmt/golangci-lint/gitleaks on PATH.
Implement together with T3 (it is one of the invariant FRs).

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

**Reconciliation with ADR-0013 (conflict check).** Found before seeding:
- **C1 — artifact (PR #2).** `FR-registry.yml` `status` enum lists `waiver`, but T1
  banned waivers → change to `active | superseded`; record the T1 policy in the
  header.
- **C2 — ADR gap (PR #2, spec edit).** ADR-0013 doesn't state T1 (automated-only
  coverage; no `manual`/`waiver`; human testing = feedback, not coverage). The
  registry would enforce a rule the ADR omits → add a short T1 clause to ADR-0013.
- **C3 — spec→FR violation (RULES edit).** The hermetic-gate invariant (T2/Q2.3)
  cites *no* spec clause, but ADR-0013 forbids an FR "not grounded in a spec
  clause" and verify's spec↔FR joint would flag it. → add the hermetic invariant to
  **RULES §5** (a named authoritative source) FIRST, then `FR-INV-*` cites it.
- **C4 — enforcement tiering.** "Enforced day one" (Q3.3) = advisory in `make gate`
  + blocking at the phase/merge boundary, **never per-commit** (ADR-0013 explicit).
- **C5 — binding interpretation.** With Q3.4 (registry-declared, no in-code
  markers), ADR-0013's "every test cites a valid FR" is implemented as
  inverse-lookup **orphan-test reporting** (advisory); "valid" = the FR exists.

Execution order once C1–C3 are approved: spec edits first (C2, C3 — the `spec →`
must exist before its FR), then seed FRs, then build `verify`.
