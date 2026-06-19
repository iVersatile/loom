# Spec map — threads overlaid on SPECs/FRs

> Generated 2026-06-12 (r3, post drain-proof evening) from `docs/FR-registry.yml` (`source:` joints, `tests:`
> coverage) and `docs/OPEN-THREADS.md` (status markers, `Pointers:` lines).
> Edge types are judgment calls, reviewed not parsed. Regenerate on ask
> (`/specmap`, sibling of `/achievements`).
>
> **Legend** — node fill: green = FRs present & covered (done) · yellow =
> spec live, implementation/FRs pending (in progress) · red = spec'd but
> untouched by implementation. Grey dashed nodes = threads (the "shadow"
> layer): ✅ resolved · 🟢 decided/in-build · 🟡 open. Edges: dotted =
> overlap / part-of · labeled ✕ = blocker / precursor. No edge = no spec
> relationship.

```mermaid
flowchart LR
  classDef done fill:#1a7f37,color:#fff,stroke:#0f5223
  classDef inprog fill:#bf8700,color:#fff,stroke:#7a5600
  classDef untouched fill:#cf222e,color:#fff,stroke:#82071e
  classDef shadow fill:#57606a,color:#ddd,stroke:#24292f,stroke-dasharray: 5 4

  subgraph SV["SPEC-verbs"]
    detect["detect (2 FR ✓)"]:::done
    plan["plan (3 FR ✓)"]:::done
    build["build (11 FR ✓)"]:::done
    exec["exec (3 FR ✓)"]:::done
    shell["shell (1 FR ✓)"]:::done
    teardown["teardown (4 FR ✓)"]:::done
    doctor["doctor (4 FR ✓)"]:::done
    logs["action+diag log (2 FR ✓)"]:::done
    bootstrap["entry: bootstrap (4 FR ✓)"]:::done
    update["update (spec'd, 0 FR)"]:::untouched
    import["import (staged)"]:::untouched
    export["export (later)"]:::untouched
  end

  subgraph SP["SPEC-playbook"]
    layers["layer resolution (4 FR ✓)"]:::done
    lock["lockfile (1 FR ✓)"]:::done
    basepb["base playbook (1 FR ✓)"]:::done
    frozen["frozen decisions (1 FR ✓)"]:::done
    harness["harness: incl. trust (4 FR ✓)"]:::done
    shellcfg["shell config model (1 FR ✓)"]:::done
  end

  subgraph RG["RULES §5 / guardrails"]
    inv["AI-first invariants (4 FR ✓)"]:::done
    guards["guard hooks (4 FR ✓)"]:::done
  end

  T5["T5 ✅ lock fidelity"]:::shadow -.- lock
  T6["T6 ✅ dry-run mutation"]:::shadow -.- plan
  T7["T7 ✅ home re-sync"]:::shadow -.- build
  T8["T8 ✅ agents: install"]:::shadow -.- build
  T9["T9 🟢 entry verbs (exec+shell shipped — header stale)"]:::shadow -.- exec
  T9 -.- shell
  T19["T19 ✅ gate dep pin"]:::shadow -.- lock
  T1T2["T1/T2 ✅ test doctrine"]:::shadow -.- inv
  T3["T3 🟢 FR seeding"]:::shadow -.- inv
  T4["T4 ✅ PATH owner (FR-BUILD-011)"]:::shadow -.- shellcfg
  T16["T16 ✅ harness home (ADR-0015, engine landed)"]:::shadow -.- harness
  T29["T29 ✅ segment-aware guard (shared splitter)"]:::shadow -.- guards

  T10["T10 🟢 non-root (design + user: clause AUTHORIZED 2026-06-12; red-team gates PR2+)"]:::shadow -->|"✕ user: clause = PR 2"| harness
  T10 -->|"✕ role marker = PR 4"| guards
  T20["T20 🟡 egress decided: observe→enforce (item 009 waits on trial)"]:::shadow -->|"✕ fetch amendment"| bootstrap
  T20 -->|"✕ future network: field"| SP
  T15["T15 🟡 auth decided: C′→D (item 008 waits on trial)"]:::shadow -->|"✕ post-trial"| harness
  T18["T18 🟡 multi-agent perms"]:::shadow -->|"✕"| harness
  T28["T28 🟡 harness self-defense (trust-config class landed; 044 ruled d+e+a-lite)"]:::shadow -.- guards
  T28 -->|"✕ trust-source digest pinning"| lock
  T12["T12 🟢 single dev container"]:::shadow -->|"✕ topology"| build
```

## Workstream overlay (see docs/WORKSTREAMS.md)
The diagram's subgraphs already align with the project workstreams — this overlay
names the mapping so the spec-map doubles as a per-stream coverage view. Full FR/ADR
index: `docs/WORKSTREAMS.md`.

- **SV (SPEC-verbs)** → **The Spine** — detect/plan/build/exec/shell/teardown/doctor/
  bootstrap (+ the red `update`/`import`/`export` backlog). `logs` → **Verification**
  (observability); `doctor` is a Spine verb that *serves* Verification.
- **SP (SPEC-playbook)** → **The Spine** — layers/lock/basepb/frozen/harness/shellcfg.
- **RG (RULES §5 / guardrails)** → `inv` (AI-first invariants) → **AI-First**;
  `guards` → **Guardrails**.
- **Threads by stream**: T20/T28 → Guardrails · T10 → Guardrails + The Spine ·
  T15 → The Run (creds) · T18 → AI-First/Dogfood (multi-agent perms) ·
  T12 → Target Env (topology).
- **Invisible here BY DESIGN = the coverage signal**: **The Run** and **Target Env**
  hold ADRs but **zero FRs** (designed-not-built), and **Dogfood** lives in RULES/TEAM
  — so they cast no spec shadow. Their absence from this map *is* the gap (see
  WORKSTREAMS.md "FR/ADR coverage").

⚑ The diagram's FR counts are STALE (e.g. `build` shows 11, registry has 17; `doctor`
4 vs 6) — a `/specmap` regen is due; the stream overlay above is the durable lens.

## No spec shadow (by design or gap?)

`T21` transport (mechanism PROVEN live 2026-06-12 — header still says "in
build") · `T22` auto-trial · `T23` AUTOPILOT scoping · `T24` /achievements ·
`T25` context economy · `T26` rollback recommendation · `T27` autopilot
control+observability (phase-1 watchdog→page queued) · `T17` activity
scripts — governance/process threads with no SPEC/FR projection. Standing
question each regeneration: which graduate from TEAM.md convention to spec?
T21 remains the likeliest (proven mechanism, outgrowing dogfood); T27's
wake verb (`loom poke`) is a new candidate — if it lands engine-side it
enters SPEC-verbs.

## Reading the shape

- **The yellow band cleared today**: r2's two in-progress nodes both went
  green — `harness:` finished its engine work (#86/#87/#107) and grew the
  trust clause + materialization (#113, FR-BUILD-013/014, FR-SCHEMA-008);
  `shell` shipped (#85, FR-SHELL-001). A new green section appeared:
  `shell config model` (T4, FR-BUILD-011). The spine is now **54 active
  FRs, all tested** — ADR-0013's automated-only doctrine still holding.
- **Red is honest backlog**: `update`, `import`, `export` — spec'd verbs
  with no implementation claim, Phase 2+ scope.
- **The guard family is the active frontier**: guards grew to 4 FRs
  (T29's segment-aware evaluation + the trust-config protect class), and
  T28's ruled mechanism (044 = build-time digest pinning) points at the
  **lockfile** — trust-source digests join what the lockfile pins, the
  first time a security thread has driven a contract section.
- **T10 is no longer the undecided blocker**: design drafted, SPEC
  `user:` clause human-authorized 2026-06-12 (adv-048); advisor red-team
  gates PR 2+. Its two edges (harness materialization, guard role marker)
  are now scheduled work, not questions.
- **Two edges still wake on the trial verdict (2026-06-18)**: T20 egress
  and T15 auth envelopes self-promote at the verdict.
- **Header-rot flag for the audit**: T9 and T21 markers lag reality (both
  effectively done/proven); the map renders ground-truth markers, so they
  read stale here by design — the fix belongs in OPEN-THREADS.
