# T11 — container name `loom-loom-dev` is awkward (doubled "loom")   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

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
