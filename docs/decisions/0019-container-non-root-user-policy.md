# ADR-0019 — Container runs as a configurable, default-root non-root user
**Date:** 2026-06-13   **Status:** Proposed (authorship chain: **human-decided** — the `user:` clause was drafted by the agent, red-teamed by the advisor [PASS-with-amendments, 2026-06-13, envelope 047], the one amendment [dropping the hard `uid 1000` from the frozen text] routed back and **re-authorized by the human in-session** — → **agent-transcribed** — this ADR + the PR-2 schema/merge/validate + plumbing — → **human-accepted**; acceptance = PR merge, per RULES §5 / C3)

## Context
`loom-loom-dev` runs as `root@/root` (`docker exec … whoami` → root). That is
why every dotfile materializes to `/root`, and it is one of the three gates the
full-auto re-evaluation lists (TEAM.md, HARNESS.md): root in the container kills
the in-container-iptables egress option (T20) and widens the blast radius of any
prompt-injected agent. A non-root `dev` user (conventional for dev containers,
aligned with the retired `devenv`) reduces that radius and makes the home target
explicit.

The engine hardcodes the home (`containerHome = "/root"`, `internal/engine/
container.go`), runs `docker run` with no `--user`, syncs `$HOME` via `docker cp
…:/root/`, and provisions as root. The drain-inbox role guard
(`.claude/hooks/drain-inbox.sh`, LL-011) falls back to `id -un == root ⇒
loom-author` — a guess that breaks the moment the container is non-root.

T10 PR 1 (#122) already forced every in-container home path through the single
`containerHome` owner (grep-guarded) so this change retargets one value, per
ADR-0016's consequence ("T10 retargets entry by changing one configured-user
value").

## Decision
1. **`user:` is declared playbook config — an optional, last-non-empty-wins
   scalar, authored at ANY tier.** Unset means root, so every pre-T10 playbook
   is unchanged. `$HOME` derives: `root → /root`, any other `<user> →
   /home/<user>`. *Not* base-only like `harness.settings:` — an env-wide base
   default with a per-project override is the legitimate shape. The frozen
   clause text lives in SPEC-playbook (`#user`), human-reauthorized 2026-06-13.
2. **Provision-as-root / run-as-user split — entry verbs pass `-u <name>` BY
   NAME.** *(Amended 2026-06-13 — advisor red-team ruling Model A on loom-author
   draft 050; human re-accepted in-session. Supersedes the original
   "`docker run --user`, no `-u` flag" form, which was causally impossible: see
   below.)* The container **runs as root** (no `docker run --user`); the
   configured non-root user is created at provision (decision 4), and **entry
   verbs (exec/shell) run as it via `docker exec -u <user>`, keyed on the
   NAME**. Provisioning (apt-get, `/usr/local/go`, `/var/lib/loom` sentinels)
   runs as the container's root default.
   - **Why not `docker run --user`** (the original form): `docker run --user` is
     fixed at container *create* and requires the user to *already exist* — but
     the collision-tolerant `useradd` (decision 4) runs at *provision*, AFTER
     create. You cannot pin the uid at create and pick a free one at provision;
     on a base shipping uid 1000 (node images — decision 4's own case),
     `--user 1000` runs as the WRONG existing user and `useradd -u 1000` then
     fails. Keying entry verbs on the NAME (which exists post-provision)
     resolves the ordering and keeps decision 4's next-free uid intact.
   - **Hardening scope:** T10's target is the **agent's** blast radius — entry
     verbs (the agent session) run non-root. The idle PID 1 (`sleep infinity`)
     running as root is an accepted Phase-1 surface; full PID-1 non-root is a
     later hardening if a threat requires it (would revisit toward Model B,
     `docker run --user 1000` + collision=error).
3. **Ownership chown after home-sync, scoped — `docker cp -a` is NOT a
   substitute.** `docker cp` writes root-owned files; the run-as-user agent
   cannot read its own home without an ownership fix. `cp -a` preserves the
   *host* numeric uid (the wrong owner). The chown is scoped to the materialized
   file set (`res.Materialized`), never a blanket `chown -R $HOME` — a blanket
   `-R` walks into the read-only `.credentials.json` bind and errors.
4. **uid 1000 preferred, not hard-pinned.** The contract is "a non-root user
   named `<user>`," uid 1000 when free, next-free + log on collision (some base
   images — node — already ship uid 1000). The `container:user` doctor claim
   keys on **name** (`id -un`), not uid, so name-based verification supports
   this with no change.
5. **A root-owned role marker replaces the uid guess.** Provision writes
   `/var/lib/loom/role` (`0644`, root-owned) from `ContainerSpec.Role`. The drain
   guard resolves the marker → UNRESOLVED = no-op (fail-closed). Security gain,
   not parity: a non-root agent cannot forge its own role, and a host-side advisor
   session has no marker, so the LL-011 fail-closed floor holds.

   **§5 amendment (2026-06-15, PR4 Part-1 REDO — LL-014, envelope adv-062;
   human-authorized spec change, merge = acceptance).** Part 1 (#154) shipped the
   marker but sourced the role from the ambient `LOOM_SESSION_ROLE` env, no-op'd
   silently on an empty role, and wrote only behind the convergence early-return —
   so a normal `loom build` never produced it and a verify was "passed" by a
   hand-run `docker exec` (drift, not convergence). The amendment makes the marker
   producible the loom way:
   - **Declarative in-tree source: an optional `role:` playbook field** (SPEC-
     playbook `#role`), mirroring `user:`. This **supersedes** the original "never
     a playbook `role:` key" wording above — the role model still lives at the TEAM
     /ADR-0021 layer, but its *value per environment* must be tree-recorded to be
     reproducible. `loom.yml` (loom-dev) sets `role: loom-author`.
   - **`LOOM_SESSION_ROLE` is demoted** to an explicit override / test-seam (wins
     over `role:` when set — lets a second seat on a shared tree override without
     editing the playbook).
   - **Fail loud, not silent:** a non-root `user:` with an empty/invalid `role:` is
     a HARD build ERROR; root + empty role is a visible warning (no marker, root
     fallback intact).
   - **Convergence dimension:** the marker joins the early-return digest
     (`needsRoleMarker`, mirroring `needsHomeSync`/`/var/lib/loom/home`), so a
     missing/stale marker self-heals on the next plain `loom build` on every env.

## Consequences
- **Default-root compatibility:** unset `user:` is byte-identical to today; the
  change is inert until a playbook opts in.
- **One known edge (for this ADR, deliberately un-special-cased):** since
  unset = root, a later layer setting `user: root` silently re-grants root.
  Root-drop is enforced at the full-auto re-evaluation gate, not in the scalar
  merge — Phase 1 does not special-case the merge.
- **Migration:** the `user:` value rides the provision sentinel digest, so a
  change re-provisions an existing container; the agent-home volume carries
  root-owned files from the root era. Two distinct chowns (do not conflate):
  decision 3's *per-sync* chown covers the materialized set (`res.Materialized`);
  a *one-time provision* `chown` migrates the carried volume's root-era files to
  the user — both **excluding the read-only `.credentials.json` bind** (a blanket
  `-R` errors on it). One recreate validates this migration and 039's trust-flag
  durability.
- **Slicing (T10 thread):** PR 1 (#122, landed) = single-owner home fix.
  **PR 2 (this ADR) = the `user:` clause + schema/merge/validate +
  `ContainerSpec.User/Home` plumbing** (resolved value populated, engine not yet
  consuming it). PR 3 = engine behavior (decisions 2–4: container runs as root +
  entry-verb `-u <user>` by name, collision-tolerant `useradd`, scoped chown,
  integration-test re-derivation). **PR 4 = the role marker (decision 5 + §5):
  Part 1 = engine — declarative `role:` field, convergence-folded write, fail-loud
  (#154, redone the loom way per §5/LL-014) + the `container:user` doctor claim;
  Part 2 = the drain-guard swap that READS the marker (human-applied, trust path).**

## Links
- T10 thread (docs/OPEN-THREADS.md) — full design + advisor red-team verdict.
- ADR-0015 (harness home / `$HOME` materialization rule), ADR-0016 (entry verbs
  decision 7: entry runs as the configured user), ADR-0018 (declared-config
  doctrine the `user:` scalar follows).
- SPEC-playbook.md `#user`, `#role` — the frozen clauses. FR-SCHEMA-009 —
  schema/merge/validate + `$HOME` resolution coverage. FR-BUILD-016 — the role
  marker (declarative source, convergence write, fail-loud). LL-014 — why the
  marker must be tree-produced, not hand-written.
