# T13 — `loom-dev` has no project/repo mount   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

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
