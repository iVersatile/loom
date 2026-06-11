# T1 — Manual-test ban for required FRs   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

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
