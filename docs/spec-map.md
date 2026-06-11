# Spec map — threads overlaid on SPECs/FRs

> Generated 2026-06-11 (r2, post trial-flip) from `docs/FR-registry.yml` (`source:` joints, `tests:`
> coverage) and `docs/OPEN-THREADS.md` (status markers, `Pointers:` lines).
> Edge types are judgment calls, reviewed not parsed. Regenerate on ask
> (recurrence candidate: `/specmap`, sibling of `/achievements`).
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
    plan["plan (2 FR ✓)"]:::done
    build["build (8 FR ✓)"]:::done
    exec["exec (3 FR ✓)"]:::done
    teardown["teardown (2 FR ✓)"]:::done
    doctor["doctor (1 FR ✓)"]:::done
    logs["action+diag log (2 FR ✓)"]:::done
    shell["shell (staged, 0 FR)"]:::untouched
    bootstrap["entry: bootstrap (clause #64 ✓, FRs queued: item 010)"]:::inprog
    import["import (staged)"]:::untouched
    export["export (later)"]:::untouched
  end

  subgraph SP["SPEC-playbook"]
    layers["layer resolution (4 FR ✓)"]:::done
    lock["lockfile (1 FR ✓)"]:::done
    basepb["base playbook (1 FR ✓)"]:::done
    frozen["frozen decisions (1 FR ✓)"]:::done
    harness["harness: section (clause #53 ✓, engine 0 FR)"]:::inprog
  end

  subgraph RG["RULES §5 / guardrails"]
    inv["AI-first invariants (4 FR ✓)"]:::done
    guards["guard hooks (3 FR ✓)"]:::done
  end

  T5["T5 ✅ lock fidelity"]:::shadow -.- lock
  T6["T6 ✅ dry-run mutation"]:::shadow -.- plan
  T7["T7 ✅ home re-sync"]:::shadow -.- build
  T8["T8 ✅ agents: install"]:::shadow -.- build
  T9["T9 ✅ entry verbs"]:::shadow -.- exec
  T9 -.- shell
  T19["T19 ✅ gate dep pin"]:::shadow -.- lock
  T1T2["T1/T2 ✅ test doctrine"]:::shadow -.- inv
  T3["T3 🟢 FR seeding"]:::shadow -.- inv

  T16["T16 ✅ harness home (ADR-0015)"]:::shadow -->|✕ engine next| harness
  T10["T10 🟡 non-root"]:::shadow -->|✕| harness
  T10 -->|✕ revisit role-guard fallback| guards
  T20["T20 🟢 egress decided: observe→enforce (item 009 waits on trial)"]:::shadow -->|✕ fetch amendment| bootstrap
  T20 -->|✕ future network: field| SP
  T15["T15 🟢 auth decided: C′→D (item 008 waits on trial)"]:::shadow -->|✕ post-trial| harness
  T18["T18 🟡 multi-agent perms"]:::shadow -->|✕| harness
  T4["T4 🟡 PATH owner"]:::shadow -->|✕ field candidate| SP
  T12["T12 🟢 single dev container"]:::shadow -->|✕ topology| build
```

## No spec shadow (by design or gap?)

`T21` transport · `T22` auto-trial · `T23` AUTOPILOT scoping · `T24`
/achievements · `T17` activity scripts — governance/process threads with no
SPEC/FR projection. Standing question the map raises each regeneration:
should any of these graduate from TEAM.md convention to spec (T21's
transport is the likeliest candidate once it outgrows dogfood).

## Reading the shape

- **The spine is green**: every built verb and every playbook schema section
  has covered FRs — 41 FRs, all `active`, all with tests (ADR-0013's
  automated-only doctrine holding).
- **The yellow band is the live frontier**: `harness:` (clause accepted,
  T16 engine work queued next) and `entry: bootstrap` (clause accepted +
  conformance-fixed #64; FR extraction dispatched, inbox item 010).
- **Red = declared, not started**: `shell` (staged behind T16), `import`/
  `export` (Phase 2+). Red here is honest backlog, not neglect.
- **The frontier's gates are now decisions, not questions**: T20
  (observe→enforce) and T15 (C′→D) were both decided 2026-06-11; their
  build envelopes (items 009/008) self-promote at the trial verdict
  (2026-06-18). T10 non-root remains the only undecided blocker on the
  map; T18 folds into the harness: engine work.
- **Trial note**: the auto-trial went LIVE 2026-06-11 — still castless
  on this map (governance), but its verdict is the wake-signal for two
  of the edges above.
