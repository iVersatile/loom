# ADR-0021 — Role resolution under multi-role-per-container
**Date:** 2026-06-14 (revived + **Accepted** 2026-06-17 for T34)   **Status:** **Accepted — Option A** (human, 2026-06-17). The build (per-session routing-role mechanism, marker = trust-union, drain/statusline keying) is advisor-in-loom Phase 3 Slice A, gated on this acceptance.

## Acceptance — Option A (human, 2026-06-17)
**Accepted: Option A** — split per-session `session-role` (routing/UX, agent-settable, advisory) from per-container root-owned `trust-role` marker (carrying the union of roles the container may act as); security guards key on `trust-role` only; a session may **narrow within** but never **widen** the container's trust set. Chosen because it is the enabler for advisor-in-loom (T34) — advisor + ephemeral author sessions in one `loom-dev` — and it leaves the LL-011 fail-closed floor untouched (trust never rides an agent-writable signal). **B rejected** (re-inherits the launch-bound footgun) · **C rejected** (needs a root-side per-session write today's non-root topology can't cleanly provide).

**Reversibility — impact of switching A → B later (asked at acceptance):** A and B share the **same trust model** — the root-owned marker is the floor in both, narrow-not-widen. They differ only in the **routing** layer: A adds a per-session, live-switchable routing-role; B is launch-bound env. So **A → B is a *subtraction*** — drop the per-session routing file, revert routing to the launch-bound marker/env. **No trust/security change, no data migration**; the only loss is re-inheriting B's weakness (role launch-bound, relaunch-to-change, wrong default when unset). The reverse (B → A) is the *expensive* direction — it means *building* the routing layer A already provides. So **A is the low-regret pick**: start at the superset, simplify to B cheaply if live-switch is never used. **Caveat:** A → B stays cheap only while nothing hard-depends on live role-switching (e.g. a session flipping advisor↔author mid-run) — avoid creating that dependency.

> **Revived for T34 (advisor-in-loom), 2026-06-17.** This ADR was drafted 2026-06-14 then held (#145) — it is exactly the decision the **advisor-in-loom** workstream now needs: an advisor session **and** ephemeral author sessions co-residing in ONE `loom-dev`. Option A (now accepted) is the enabler.

## Context
ADR-0019 decision 5 and ADR-0020's drain-guard fixed role resolution as a
layered lookup: **`LOOM_SESSION_ROLE` env → `/var/lib/loom/role` marker →
UNRESOLVED = fail-closed no-op** (LL-011 floor). The marker is *per-container*
(root-owned, `0644`, one scalar) — it answers "what is this container's role,"
and that shape silently assumes **one role per container**.

The 2026-06-14 ~22:00Z workstream checkpoint recorded exactly that as the target
("ONE-ROLE-PER-CONTAINER is the target; multi-role NOT needed; human-confirmed")
and reframed the marker as *the* loom-native role mechanism, with
`LOOM_SESSION_ROLE` a temporary bridge.

**This ADR records a reversal of that checkpoint.** Driver: *harness-container
reality* (human-selected, 2026-06-14). The agent sessions run inside a harness
container that is **distinct from `loom-dev`**, and in that container **more than
one role co-resides** (e.g. an advisor session and an author session in one
container). A per-container marker holds exactly one value, so it **physically
cannot distinguish two sessions' roles** in the same container. Multi-role-per-
container is therefore a real requirement, not optional polish — and the current
primary signal, launch-bound `LOOM_SESSION_ROLE`, is a stopgap: it cannot be
injected into a live session and defaults wrong (✍️/loom-author) when unset.

This forces a question ADR-0019 decision 5 did not have to answer: **when role
must be per-session, which layer is primary, and is the per-session signal a
*trust* signal or only a *routing* signal?**

## The load-bearing distinction
Role is used for two unrelated jobs, and conflating them is the trap:

| Job | Granularity it wants | Forgeability tolerance |
|---|---|---|
| **Routing/UX** — statusline glyph, which inbox a session drains, who relays git-tasks | per-**session** | advisory; a wrong value is a cosmetic/workflow bug |
| **Trust** — whether the drain-guard fires, what privileged action a container may take (LL-011 fail-closed floor, ADR-0017 writer-trust) | per-**container** | must be **unforgeable** — a non-root agent forging its role defeats the floor |

A single mechanism cannot be both *per-session live-settable* and *root-owned
unforgeable*. So the resolution stays layered; the design choice is **which
job each layer serves**, not "pick one mechanism."

## Decision (deferred — choose one)
The recommendation is **Option A**; the human decides by accepting one option's PR.

### Option A — Split the concept: per-session *routing-role*, per-container *trust-role* (recommended)
- **`session-role`** (routing/UX): a per-session signal the harness reads each
  turn — a session file (e.g. `$CLAUDE_SESSION_DIR/role`) or `LOOM_SESSION_ROLE`
  as its seed. Drives the statusline glyph and inbox routing. **Agent-settable,
  advisory, live-changeable** (fixes the launch-bound footgun).
- **`trust-role`** (security): the existing root-owned `/var/lib/loom/role`
  marker, now explicitly the *container* trust anchor. Multi-role container ⇒ the
  marker carries the **union** of roles the container is provisioned to act as.
- **Rule:** security-sensitive guards (drain-inbox, any privileged verb) key on
  **`trust-role` only**; `session-role` may **narrow within** the container's
  trust set but **never widen** it. A session can *say* it is the advisor for
  routing, but cannot *act* beyond what the marker grants.
- *Resolution (routing):* session-file → `LOOM_SESSION_ROLE` → marker → default.
  *Resolution (trust):* marker → fail-closed (unchanged from ADR-0019/0020).
- **Pro:** multi-role works; live role-switch with no relaunch; the LL-011 floor
  is untouched (trust never rides an agent-writable signal); loom-native, non-
  root-safe. **Con:** introduces an explicit two-name model and a new per-session
  file the harness must write.

### Option B — Env primary, accept relaunch-to-change
- Keep `LOOM_SESSION_ROLE` as the single primary; marker = fallback/default.
  Changing role = relaunch the session.
- **Pro:** simplest, half-built already, no new mechanism. **Con:** the launch-
  bound weakness that drove this ADR survives; unset still defaults to author;
  and a single agent-settable env as the *trust* signal weakens LL-011 unless
  the drain-guard still defers to the marker — in which case this is Option A
  without the clean split.

### Option C — Per-session root-owned sub-marker (`/var/lib/loom/role.<session>`)
- Keep the marker primary but make it per-session: provision/a root hook writes
  `role.<session-id>` at session start, unforgeable like the container marker.
- **Pro:** keeps an unforgeable floor *and* per-session granularity. **Con:**
  who writes it? Inside a non-root container the session is not root, so this
  needs a root-side trigger at every session start — the same create-time vs
  provision-time ordering hazard ADR-0019 decision 2 hit with `docker run
  --user`. Likely causally hard in today's topology; revisit if a root-side
  per-session hook becomes cheap.

## Alternatives considered
- **Drop the marker, env-only** — rejected: removes the unforgeable floor;
  ADR-0019 decision 5 added the marker precisely so a non-root agent cannot
  forge its role.
- **One container per role (hold the checkpoint)** — this is the status quo the
  reversal overrides; recorded as rejected *for the harness-container topology*
  because that topology co-resides roles by construction. It remains correct for
  `mac-dev-topology` (one role in `loom-dev`), so Option A must keep the single-
  role container byte-identical when only the marker is present.

## Consequences (positive / trade-offs / revisit-if)
- **Positive (Option A):** the statusline role glyph resolves without ad-hoc env;
  live role-switch; the trust floor is *strengthened* by being named separately
  from routing.
- **Trade-off:** a second role concept to document (SPEC + HARNESS.md) and a
  harness contract to write the session-role file. Two-name models invite
  conflation — the "narrow-not-widen" rule is the guardrail and must be tested.
- **Compatibility:** single-role containers (`loom-dev`) must stay inert — with
  no session-role set, resolution falls through to the marker exactly as today.
- **Revisit-if:** a root-side per-session hook becomes available (reopens Option
  C); or PID-1 goes non-root (ADR-0019 decision 2 hardening), changing who can
  write markers.
- **Supersedes:** the role-mechanism framing in the 2026-06-14 checkpoint and
  narrows ADR-0019 decision 5 / ADR-0020's drain-guard from "the marker is *the*
  role mechanism" to "the marker is the *trust* role; routing-role is separate."
  The existing PR4 role-marker work (`docs/patches/0022`) stays valid — it builds
  the `trust-role` layer either way.

## Links
- ADR-0019 (decision 5: root-owned role marker), ADR-0020 (drain-guard role
  resolution), ADR-0017 (writer remote-trust split).
- docs/TOPOLOGY.md — `ai-user-topology` (roles co-resident is the north-star
  shape this generalizes toward); the harness-container topology should be named
  here once this ADR is accepted.
- LL-011 (drain role-guard fail-closed). T10 PR 4 / docs/patches/0022 (the
  trust-role marker build slice).
