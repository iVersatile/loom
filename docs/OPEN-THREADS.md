# Open design threads

Working record of in-progress design discussions not yet decided. **Not an ADR** —
when a thread resolves it promotes to an ADR (RULES §3) or a spec edit, and the
entry is marked Resolved with a pointer. Kept current so an interrupted discussion
resumes without losing context.

Status: 🟡 open · 🟢 recommendation drafted, awaiting decision · ✅ resolved.

---

## T1 — Manual-test ban for required FRs   🟢
Origin: ADR-0013 / the FR registry's `status: waiver` placeholder.
Question: may a **required** FR be "covered" by manual testing, and if a behavior
is genuinely un-automatable, what happens?

**Evaluation (through the AI-first, self-evolving lens).**
- North star = no human in the loop. "Manual testing" presupposes a human, so for
  a required FR it means *never verified* — a permanent blind spot the self-evolving
  agent cannot re-check on each change. Incoherent with the north star.
- A `waiver` is a **trust** artifact ("tested manually, trust me"), which violates
  ADR-0005 (guardrails by mechanism, not trust). The design test — "would the
  guardrails hold if the agent tried the worst thing?" — flags it: an agent could
  waive an FR to bypass verification.

**Recommendation — strong ban + reclassify, not waive:**
1. A required FR's only valid coverage is an **automated** test. No `manual`.
2. A genuinely un-automatable behavior is **not** a required FR — reclassify it:
   - prefer an **automatable proxy** (test the mechanism — e.g. "bootstrap runs to
     completion in a clean container" instead of "a human bootstraps on bare
     metal"); most "un-automatable" FRs are only un-automatable *as literally
     stated*; or
   - **downgrade** to a non-required/advisory FR with a manual checklist that
     explicitly does NOT count toward "done".
3. `waiver` only as a rare last resort: requires review, an **expiry**, and `verify`
   must **loudly report** it as a known blind spot. Preferred path is always
   proxy-or-downgrade.

**Residual decision:** accept "no waivers — proxy/downgrade only", or keep `waiver`
as the rare expiring exception in (3)?

---

## T2 — Hermetic unit gate   🟡
Origin: LL-006 / LL-008 — three CI reds this session were unit tests that passed
locally and failed in CI because the local box lacked an env var / a binary CI had.
Goal: the unit tier yields the **same result regardless of host tooling/env**, so
the local gate and CI gate test the same thing (catch the class before CI).

Open questions:
- **Q2.1 Enforcement point.** (a) local `make gate` scrubs the env by default so
  local ≡ CI *[lean — most AI-first; the agent's local gate IS the gate]*; (b) a
  separate CI `unit-hermetic` job; (c) both.
- **Q2.2 Detection mechanism.** (a) run the unit suite in a deliberately hostile
  minimal env — docker hidden, `LOOM_*`/`ALLOW_*` unset, `PATH` trimmed to the
  gate's own toolchain *[lean — mechanism, not trust]*; (b) a static lint flagging
  tests that read `os.Getenv` without `t.Setenv`; (c) both.
- **Q2.3 Self-enforce?** Should "the unit gate is hermetic" itself be an invariant
  FR (`FR-INV-*`) in the registry, enforced by the same machinery? *[lean — yes;
  dogfoods the registry]*

---

## T3 — Phase-1 FR seeding (the bootstrap retrofit)   🟡
Origin: ADR-0013 timing / bootstrap exception. Extract FRs from the frozen Phase-1
contracts + invariants + guardrail hooks, link existing passing tests, into
`docs/FR-registry.yml` (currently `requirements: []`).

Open questions:
- **Q3.1 Authoring.** Agent drafts the seed FRs for your review *[lean — ADR-0013
  "agent-with-review"]* / you author / pair.
- **Q3.2 Seed scope.** SPEC-verbs behavioral + the global invariants + the 3
  guardrail hooks only; OR also SPEC-playbook schema/resolution FRs (layer merge,
  `rules:` union, `dotfiles:` later-wins) *[lean — include them; frozen Phase-1
  surface with tests]*.
- **Q3.3 Sequence.** Build the `verify` check **alongside** seeding so the registry
  is enforced from day one *[lean]*, vs seed first (advisory) then build `verify`.
- **Q3.4 verify's binding.** Confirm the FR→test link is **registry-declared**
  (test names/suites in the YAML, resolved against a `go test -json` run), not
  in-code markers. *[lean — matches the locked format]*
