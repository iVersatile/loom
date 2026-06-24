# T38 — Role-model generalization: identity + layered boundaries   🟢 recommendation drafted (cross-seat converged 2026-06-24; pending human acceptance)

> **Design draft — NOT scoped, NOT an ADR.** Captures the 2026-06-24 advisor↔human
> discussion so it resumes without loss. Promotes to an ADR (and gates D3) only on an
> explicit human go-ahead. D3 (role-validation / base-default-role) is **PARKED**
> behind this thread.

## What triggered it
D3 began as a 6-line spec/code drift fix (what happens when `role:` is empty). But a
human scenario broke the two-role assumption it rested on: **project-ABC** with roles
`planner` (writes specs → board), `dev` (picks up → pushes branch → auto-merge to
staging), `supervisor` (cherry-picks features → PROD), **plus** `DB-owner` (manages
schema/DDL) and `data-manager` (data/DML only). loom today hardcodes exactly two roles
(`loom-author`, `loom-advisor`) across ~7 mechanism files. Can richer roles resolve?

## The principle (human, 2026-06-24)
**A loom role should be a generic, forge-proof *identity*. loom prescribes as little as
possible about what that identity can *do*; each boundary is applied by the layer that
owns it, keying off the identity.**

### The bright line (keeps "generic" from becoming unsafe)
"Generic/flexible" means the role is a generic identity that **later layers bind
boundaries to** — NOT that loom becomes a place where arbitrary powers are declared and
loom enforces them. The DB example is the lesson: loom reimplementing DB/cloud/API authz
= a weaker, **bypassable** second copy of an authorization that already exists downstream
(connect to the DB directly and loom's copy is moot). Flexibility comes from **layering
boundaries onto a generic identity**, never from loom growing a general permission engine.
This is the existing invariant — *guardrails are mechanism, not trust* — applied to roles.

## Confirmed decisions (human, 2026-06-24)
1. **De-hardcode `loom-author`/`loom-advisor`.** Their git powers become a thin,
   **reviewed** `role → git-tier` map (trust-gated data), not names baked into ~7 guards.
2. **A fully generalized loom role = an identity that maps to a *policy across domains*.**
3. Roles remain a **union** per container (ADR-0021): the marker carries the set of roles
   the container may act as; a session narrows-within, never widens.

## The capability taxonomy — RE-DERIVED (fixing the abstraction-level error)
The first enumeration (git · shell · harness · filesystem · lifecycle · networking ·
spawn · credentials) was **flat and mixed levels** — e.g. `shell/exec` is not a sibling
of `git`; it is the *universal lever* that can reach git, filesystem, and network. A
capability is `(action × object)`, and the load-bearing axis is **loom's relationship to
the object**. Three levels:

### Level 1 — loom's OWN substrate (loom is the authority; enforces fully; no downstream)
The things loom *is*: the harness/trust config (guards, settings, the rules loom runs by),
the environment lifecycle (build/teardown/reconcile), the agent/autonomy fabric (spawn,
drain, wake, queue/inbox), the playbook/lock/frozen specs. **Most sensitive, least
delegable** — today gated human-only / trust-class (`protect-paths`, docker-actuation).

### Level 2 — the EXECUTION surface (the universal capability)
`shell` / `exec` — the raw ability to run commands in the env. **Not a sibling domain;
the meta-capability** that can reach every external resource unless a finer gate
intercepts. Granting exec ≈ granting broad reach; `guard-bash` is the in-band attempt to
gate *within* it (block recursive-delete, sudo, force-push, cred-exfil).

### Level 3 — EXTERNAL resources (loom is a GATEWAY; downstream owns the authority)
The git forge, the network, DBs, clouds, APIs, the repo-of-record. loom's job per
resource is **(a) gate the path** (egress, the push/merge guard as a *local* gate /
defense-in-depth) and/or **(b) route a scoped credential**; the **external system
enforces its own domain**. (Git is the subtle case: the forge's branch-protection is the
real enforcer; `role-push-guard` is a local defense-in-depth gate.)

**A generalized loom role**, then, is an identity whose policy is written across the three
levels — substrate: usually *no* (agent roles don't edit loom's own trust core);
execution: may it exec, how gated; external: which forge tier, which egress, **which
credentials → which downstream scopes**. The credential bundle (Level 3) is the seam where
DB-owner/data-manager/cloud-admin attach — loom routes, downstream enforces.

| project-ABC role | Level 1 | Level 2 | Level 3 |
|---|---|---|---|
| planner | no | gated | git: push-tier · creds: spec/docs |
| dev | no | yes | git: push-tier · creds: dev-DB |
| supervisor | no | yes | git: merge→PROD · creds: release |
| DB-owner | no | yes | creds: **schema-DB** (DB enforces DDL) |
| data-manager | no | yes | creds: **data-DB** (DB enforces DML) |

## Zoom-out — what is loom intended to be?
**Charter today:** an AI-first, reproducible **dev-environment** tool (detect→plan→build→
run a container-per-project, with guardrails; AI-first user; guardrails are mechanism).

**Does this direction change that?** It reframes loom as *the identity + authority +
credential-routing layer for AI agents operating across dev resources* — the "who is this
agent, what may it touch, with which keys" broker. Assessment:
- **It is a *deepening*, not a new product** — loom already *is* the thing that decides
  what an AI agent may do in its env (`role-push-guard`, `protect-paths`, the credential
  adapters). Generalizing roles makes that implicit model **explicit and extensible**.
- **The scope-creep risk is real and has a name:** crossing the bright line (loom
  *enforcing* downstream domains) turns loom into a general IAM/policy platform — a
  different product, and likely a charter non-goal.
- **Verdict:** the direction fits loom's intended identity **iff the bright line holds** —
  loom owns Levels 1–2 + *gates+routes* Level 3; downstream enforces Level 3's domains.
  Hold the line → a natural deepening. Cross it → a new product we did not set out to build.

## Open questions (NOT scoped — for the confer + human)
1. Does "generic" extend to loom's **own** git ops (de-hardcode into a reviewed
   `role→tier` map — decided yes) **and** to Levels 1–2 generally, or do Level-1 substrate
   powers stay hard human-only (likely yes — least delegable)?
2. Where is the `role→tier` / `role→cred` map stored, and how is it kept **unforgeable**
   (a protect-path trust artifact; who signs off per project)?
3. Is the git push/merge guard genuinely *defense-in-depth* over forge branch-protection,
   or load-bearing on its own? (Decides how much loom must encode about branch scope.)
4. Selective/branch-scoped merge ("supervisor cherry-picks *some* to PROD") — forge's job
   (branch-protection + CODEOWNERS) or loom's? (Earlier confer crux; leaning forge.)
5. Single-role vs union-of-roles validation shape (ADR-0021 marker is a set).

## Relationship to D3
D3 is **downstream of this thread and PARKED + human-gated.** Under this model D3 likely
*shrinks*: "valid role" relaxes to "any well-formed identity," and the only loom-side
check is whether an identity is in loom's **own** Level-2/3 maps (git-tier); downstream
boundaries never enter D3. Do not un-park D3 without explicit human go-ahead.

## Convergence (cross-seat confer, 2026-06-24 — advisor + loom-author, `q-1782329857` → `q-1782332725`)
**Winner: C (fixed capability tiers) + route-scoped-creds.** The author's independent
read did not merely agree — it **dissolved the one break that could have pushed C → B**,
and corrected three things in the draft above.

**The decisive break (why C won, not just "preferred"):** my C-break was "C can't express
selective/branch-scoped merge (supervisor cherry-picks SOME features to PROD) → slides to B."
The author showed this is **structurally not loom's to enforce**: `role-push-guard` governs only
the *local in-container command line*; it cannot govern a merge landing via the forge API from
elsewhere, or the same forge token used differently. A loom-enforced branch-scoped merge tier is
therefore a **weaker, bypassable copy** (the DB lesson, generalized) — branch-scope MUST live in
the forge (branch-protection + CODEOWNERS + environment rules). **loom's merge authority is
necessarily COARSE** ("may this seat initiate a merge at all + which scoped forge cred") — and that
coarseness is the *real boundary of what loom can enforce*, not a C ceiling. B's only advantage
evaporates; C is reinforced.

**Three model corrections (the draft taxonomy above is superseded on these points):**
1. **exec is NOT the universal lever — the container ENVELOPE (L1) is.** The frozen exec spec is
   "doors, not checkpoints": exec confers no authority beyond the container's guard envelope.
   Standing L3 capabilities (network reach, mounted cred volumes, project FS) are set by **L1
   build/create-time config** (network mode, `-v` mounts, the `:ro` cred attach that binds only at
   create) and reachable by **any** in-container process (harness, long-lived agent, backgrounded
   job) — not just a fresh `exec`. **Fix: L2 gates *invocations*; the L1 *envelope* bounds L3 reach.**
2. **Credentials are DUAL-HOMED.** Routing ("which scoped cred for which seat") is L3; the
   *injection mechanism* is **L1 and load-bearing** (ADR-0027 fail-closed: token at exec time, never
   `-e`-leaked, absent volume ⇒ exec fail-closed). The cred bundle is an L3 *seam* AND an L1
   *mechanism* — filing it pure-L3 under-protects the L1 invariant.
3. **Egress is the ONE L3 loom MUST self-enforce** (the answer to the q-328509 "non-redundant +
   non-bypassable" challenge): there is **no downstream enforcer** for "this container may not reach
   the internet" (DB has Postgres GRANT; egress has nothing). `egress: none|allowlist` is enforced
   by loom's own mechanism (`--network none` / proxy sidecar); the network namespace IS the only
   path. So egress is L3-by-resource but **L1-by-enforcement**.

**Sharpened bright line (adopt — supersedes the draft's "downstream enforces L3"):**
> loom enforces an L3 boundary **only where no downstream enforcer exists** (egress: deny-by-default
> network). Where a downstream enforcer **does** exist (DB GRANT, cloud IAM, forge branch-protection),
> loom **routes a scoped credential + gates the path, and never reimplements that domain's authz**.

Scope-creep (the charter non-goal) = the moment loom enforces an L3 domain that already has a
downstream owner. The residual loom self-enforces (egress) is small and bounded ⇒ a **deepening**,
not an IAM platform.

**The de-hardcode `role→git-tier` map — 4 binding conditions (from the author, who edits the guard
daily and notes it is brittle: it false-blocked `git push -u`, `--json mergeStateStatus`, indirection):**
1. the map is a **protect-path** (human-signed, ALLOW_TRUST_CHANGE) — else a seat maps its own
   role→merge-tier and self-elevates;
2. the **marker stays root-owned/unforgeable** (ADR-0021) — the *name* the guard looks up must be
   non-forgeable;
3. **unknown role fails CLOSED to the floor** (no-push), never a default-open lookup;
4. the lookup stays a **dead-simple exact name→tier-enum match** — NOT a policy-language parse
   (any conditionals/branch-scope/globs = B's mini-IAM rebuilt in `sh` = a fail-open bug farm).

**Residual question for the human (the one open acceptance call):** is loom *self-enforcing egress*
(the single no-downstream L3) inside the bright line, or does even that smell like scope-creep? Both
seats lean **in** (no downstream owner ⇒ loom's to own; the residual is small + bounded). Human's call.

**Status:** cross-seat converged; **NOT scoped, NOT an ADR** — promotes to an ADR + un-parks D3 only
on explicit human acceptance of (a) the C+route-creds model, (b) the sharpened bright line, (c) the
egress-residual ruling.

## Convergence round 2 — the capability model (cross-seat, 2026-06-24; `q-1782333610` → `q-1782334531`)
The human flagged that the L1/L2/L3 taxonomy mixed abstraction levels. Diagnosis (advisor):
the ladder secretly sorted by THREE criteria at once — who-owns (L1), which-mechanism (L2),
where-the-resource-lives (L3) — which is why cred read "dual-homed" and egress "L3-resource/
L1-enforcement." Advisor seed: drop the ladder; a role = **envelope + gates + credential-bundle**,
with loom's own integrity as a **constant floor**. The author named the single axis that makes
it coherent and dissolves the straddles:

**One axis: the CONTROL POINT in an action's lifecycle → a DERIVED enforcer** (is there a
downstream owner at that point?):

| Part | Control point | Acts against | Enforcer | Consequence |
|---|---|---|---|---|
| **ENVELOPE** | **create**-time | the container (network posture, mounts, cred-volume attach) | **loom** (no downstream) | loom self-enforces |
| **GATES** | **invoke**-time | a command (git push/merge tier, shell/exec limits, **spawn authority**) | **loom** (the guards) | loom self-enforces |
| **BUNDLE** | **use**-time | a downstream system (scoped DB/cloud/forge keys) | **downstream** (DB GRANT, cloud IAM, branch-protection) | loom routes key + gates path, NEVER reimplements |
| **FLOOR** | **always** | loom itself (audit, trust files, role marker, agent-fabric) | **loom**, role-invariant | non-negotiable constant |

This is ONE ladder where L1/L2/L3 was three: who-owns / which-mechanism / where-resource all
collapse into *"at which control point, and is there a downstream enforcer there."*

**The straddles dissolve (nothing is dual-homed under this cut):**
- **Credential** = two capabilities sharing a word: *"is the cred file reachable"* = create-time =
  ENVELOPE (a mount); *"what authority the key carries"* = use-time = BUNDLE (downstream-scoped).
  Coupling (the mount is a precondition for the key), not straddling.
- **spawn / reaping** = mechanism vs authority: the spawn SUBSTRATE (drain/self-wake/fabric) = FLOOR;
  the AUTHORITY to spawn = an INVOKE-time GATE (`spawn-guard` already is one). Same split as git
  (the binary is floor; push/merge authority is a gate). The split IS the rule.

**No 4th part:** time/lifecycle is not a part — it IS the axis. Audit/observability is FLOOR
(role-invariant; you can't define a role with "no audit"). read/visibility resolves to ENVELOPE
(reachability includes read via mount perms; intra-container per-seat read isolation = an OS-perms
property the envelope sets at create).

**The bright line becomes one sentence:**
> loom **self-enforces at create + invoke**; at **use** it only hands a scoped key and the
> downstream system enforces; the **floor** is constant and role-invariant.

(Egress = create-time, no downstream → loom enforces — consistent, no longer an exception. DB authz
= use-time, downstream exists → loom never touches it — consistent. The round-1 egress-residual is
subsumed.)

**The kernel zoom-out is safe IFF the control-point set is CLOSED.** loom is honestly a
capability *kernel* (envelope+gates+creds keyed by identity) — but "kernel" carries scope-creep
gravity (plugins / a policy language / per-project mechanisms = the B-style IAM platform). So the
load-bearing guardrail, generalized from round-1's "flat exact-match map, not a policy parse":
**the kernel is a CLOSED set (exactly 3 dials + 1 floor); projects parameterize IDENTITIES onto
existing dials, and NEVER declare new control mechanisms.** Closed-set kernel = a deepening of a
dev-env tool; open/extensible kernel = the IAM platform = a charter non-goal.

**Residual for the human (the single charter-level ratification):** accept *"closed control-point
set — 3 dials + 1 floor — projects key identities only, never mechanisms"* as a bright line. That
one line is what keeps loom a deepening and not a new product. On acceptance, T38 promotes to an
ADR and D3 un-parks.

## Capability buckets → what a ROLE *is* (the role-design implication)
The four buckets are **not equal** as role dials — and that asymmetry IS the role design:

| Bucket | Role-design meaning | Set per |
|---|---|---|
| **Floor** | **NOT a role dial** — constant under every role (no role can disable audit, edit trust files, or forge its identity) | baseline (nobody) |
| **Gates** | **THE per-role dial:** git-tier (none/push/merge), spawn authority, shell/exec limits — the de-hardcoded `role→tier` map | **session** (the seat) |
| **Bundle** | **a key-list:** which scoped downstream creds the role receives. `DB-owner` vs `data-manager` differ **only here** | **container** (attached at create) |
| **Envelope** | **reach needs:** egress hosts, mounts | **container** (create) |

So a **role = `{gate-tiers} + {credential needs} + {envelope needs}`** — small by construction.
Heavy authority (schema-vs-data, branch-scoped merge) lives **downstream** (Bundle) or in the
**forge**, never in the role. That smallness is the design goal, not an accident.

**The structural rule (ADR-0021 union/narrow):** Gates are **per-session**; Envelope + Bundle are
**per-container**. A container hosts a **union** of roles (root-owned marker) ⇒ its envelope = the
union of reach needs (built **once** at create), its creds = the union of key needs. Each **session
narrows to one role**; Gates enforce **per command** against the session role (narrow-within, never
widen). Floor holds for all. Two-layer design: **declare roles** (gate+cred+envelope) → **a
container picks a union** → **each seat runs as one**. Topology is a choice: container-per-role
(clean isolation) or one container hosting a union of seats.

## Consolidated role & security design
**A loom role is a generic, forge-proof IDENTITY → a small policy across one closed, control-point-
sorted capability set.** End to end:
1. **Identity** — a root-owned, unforgeable marker carrying the *union* of roles a container may act
   as (ADR-0021); a session narrows to one (`session-role`), never widens. Unknown identity ⇒
   fail-closed floor.
2. **The capability set (closed: exactly 4, sorted by control point → derived enforcer):**
   **Envelope** (create / loom) · **Gates** (invoke / loom) · **Bundle** (use / downstream) ·
   **Floor** (always / loom-constant).
3. **The bright line (one sentence):** loom **self-enforces at create + invoke**; at **use** it only
   routes a scoped key and the **downstream system enforces**; the **floor** is constant.
4. **The closed-set guardrail (charter-level):** projects parameterize **identities** onto existing
   dials; they NEVER declare new control mechanisms. Closed-set ⇒ a deepening of a dev-env tool;
   open/pluggable ⇒ the IAM platform (charter non-goal).
5. **Most of it already exists** — the floor (audit/trust/marker), egress (ADR-0028), and credential
   routing (ADR-0014/0026/0027) are built; the *only* genuinely new mechanism is the **de-hardcoded,
   signed, flat `role→git-tier` map** (one Gate).

## Proposed FR / ADR breakdown
> **PROPOSED — nothing created yet.** This is the design-doc breakdown the human asked for; it is NOT
> implementation. Creating these ADRs/FRs (and building any of it, incl. D3) awaits (a) human
> ratification of the closed-set bright line and (b) the explicit, still-outstanding D3 go-ahead.

**ADRs:**
- **NEW (proposed `ADR-0029`) — Capability model: the closed control-point set.** The four buckets,
  the control-point→derived-enforcer axis, the one-sentence bright line, and the closed-set property.
  Builds on **ADR-0021** (identity/union/narrow). The headline decision.
- **Amend `ADR-0017`** (writer remote-trust) — generalize the two hardcoded roles
  (`loom-author`/`loom-advisor`) into a signed `role→git-tier` map; today's tiers become two rows.
- **Reference `ADR-0021`** — the four-bucket model is *how* the union marker resolves to capabilities
  (per-session Gates vs per-container Envelope+Bundle).
- **Charter / RULES** — record the **closed-set bright line** as a charter-level non-goal guardrail
  ("loom is a closed capability kernel, not an open policy engine").

**FRs** (★ = new; the rest EXIST and are mapped to a bucket — showing the design is mostly built):
- **Gates / the de-hardcode (the one near-term build):**
  - ★ **FR-GUARD-TIER-001** — `role-push-guard` resolves role→git-tier by **exact-match** lookup in
    the signed tier-map; **unknown role → no-push floor** (fail-closed). *(No policy-language parse —
    flat dict only; the round-1 condition.)*
  - ★ **FR-GUARD-TIER-002** — the tier-map is a **protect-path** (ALLOW_TRUST_CHANGE) and the marker
    stays **root-owned/unforgeable**; an agent cannot self-elevate by editing the map.
  - `FR-GUARD-PUSH` (exists, #282) — becomes one row in the map (the `loom-author` tier).
  - `FR-GUARD-SPAWN-RATE` / `FR-GUARD-REAP` (exist) — spawn **authority** = an invoke-time Gate.
- **Envelope (create-time):** `FR-NET` (exists, ADR-0028) — egress is the create-time envelope;
  ★ **FR-ENV-001** (proposed) — build derives the container envelope from the **union** of hosted
  roles' reach needs.
- **Bundle (use-time):** `FR-CRED-*` (exist, ADR-0014/0026/0027) — per-role scoped-cred routing;
  downstream enforces. ★ **FR-CRED-ROLE** (proposed) — a role's declared cred needs determine which
  scoped creds attach.
- **Floor (already built):** `FR-INV-002` (audit), `FR-GUARD-PROTECT-PATHS`, `FR-GUARD-TRUST-CONFIG`,
  `FR-BUILD-016` + `FR-SCHEMA-010` (marker unforgeable) — the floor is **role-invariant**, no new FR.
- **Identity / validation (the shrunk D3):** ★ generalize the `role:` schema FR + define **"valid
  role = a name present in the signed tier-map"** (fail-closed otherwise). This is all D3 becomes.

**Near-term implementable slice (when un-parked):** the **de-hardcode tier-map** (`FR-GUARD-TIER-*`)
— small, closed, the only genuinely new mechanism; everything else exists or is the `ADR-0029`
conceptual consolidation.

## Native roles + naming (FINAL: worker / orchestrator + review-duty pin — human 2026-06-24)
> **Decided 2026-06-24:** keep `worker` / `orchestrator` (counter-offer `contributor`/`maintainer`
> declined — fleet-topology consistency chosen over forge-vocabulary precision) **AND pin the
> review duty** (the non-negotiable both seats required). Proposed pin wording below, staged for
> `ALLOW_SPEC_CHANGE`.
loom ships exactly **two** native roles (the human is the out-of-band ceiling, not a container role).
The old `author`/`advisor` names are **renamed** because they mislead: they name a *persona* (writes
vs advises), **invert the hierarchy** ("advisor" sounds weaker but holds merge+spawn), and carry none
of the real axes (floor/elevated, push/merge, ephemeral/persistent).

| was | → now | bucket meaning |
|---|---|---|
| `loom-author` | **`loom-worker`** | the fail-closed **FLOOR** / default; **ephemeral**; Gates: **push-no-merge**, no spawn |
| `loom-advisor` | **`loom-orchestrator`** | the **ELEVATED** role (opt-up via env); **persistent**; Gates: **merge** + **may spawn** the worker fleet |

**Rationale:** "one orchestrator + many workers" is the canonical pairing for ADR-0025's topology (a
persistent coordinator spawning an ephemeral fleet); names capability/relationship, not persona; correct
hierarchy. **Caveat (accepted at decision time):** "orchestrator" foregrounds spawn/coordinate over the
*merge gate* (the actually-enforced capability) and the *review* duty — accepted because the merge truth
is carried precisely by the tier-map row, so the name need not. The two become the first rows of the
signed `role→git-tier` map: `loom-orchestrator → merge` · `loom-worker → push-no-merge` · `unknown →
no-push floor`.

**MIGRATION (breaking; deferred behind ratification + the explicit D3 go-ahead — NOT done now):** the
rename touches the marker string (`/var/lib/loom/role`), ~7 guard files (`role-push-guard`,
`role-inject`, `spawn-guard`, `checkpoint-inject`, `schema.go`, `exec.go`, `doctor_rolemarker.go`),
`loom.yml role:`, `LOOM_SESSION_ROLE` values, and ADR-0017/0021/0025 prose.

**Migration — CLEAN CUT via one atomic `build --force` (cross-seat converged, `q-1782336472` →
`q-1782336693`).** The marker AND the guards are both **Envelope** — they materialize together at
build/create — so a single `build --force` recreate swaps both **atomically**: no running container
ever straddles new-guards/old-marker, so there is **no transition window**. The fail-closed floor
makes the only failure mode a *safe under-privilege* (an old/unknown marker → `*) fail-closed` →
floor; never over-privilege). **`accept-both` is REJECTED — it's the riskier option:** recognizing
`loom-advisor` as a legacy elevated *alias* keeps a second elevation name alive, and since
`LOOM_SESSION_ROLE` is a settable override, `LOOM_SESSION_ROLE=loom-advisor` would still elevate via
the alias — a *widened-trust* surface, violating narrow-not-widen (ADR-0021) for a window the atomic
rebuild already eliminates. Lean into the fail-closed floor, don't blunt it with aliases.

**Action item the rename surfaced (non-negotiable, independent of which name wins):** neither the new
name nor the tier-map carries the orchestrator's **review-before-merge duty** (review is process/trust
per RULES + designer≠builder, not a guard mechanism). The name foregrounds spawn/coordinate (the least
security-critical facet); a holder self-conceiving as "orchestrator" can under-weight the gate-keeper
mindset (rubber-stamp merges to keep the fleet moving). **Pin "review-before-merge = the orchestrator's
defining duty" explicitly in RULES / ADR-0025.** With that pinned, the name's gap is acceptable.

**Counter-offer logged for the human (author, `q-1782336693`):** `loom-contributor` / `loom-maintainer`
— the *forge-native* pair names the enforced truth more precisely ("maintainer" carries review+merge
implicitly + universally) and speaks the forge's vocabulary, consistent with the bright line (loom
delegates merge-semantics richness to the forge). Cost: drops ADR-0025's "one orchestrator + many
workers" fleet framing. Net (both seats): **either is fine; the review-duty pin is the non-negotiable,
not the word.** Human's final micro-call: `worker`/`orchestrator` (+ pin review) for fleet-consistency,
or `contributor`/`maintainer` for gate-precision.

## Review-duty pin (FINAL — proposed wording; frozen paths → staged for human `ALLOW_SPEC_CHANGE`, applied on the T38→ADR promotion)
Neither the `orchestrator` name nor the `role→git-tier` map carries the role's review obligation, so
pin it explicitly. Proposed clause, to land in **ADR-0025** (and mirror in **RULES / `docs/TEAM.md`**):

> **The orchestrator's defining duty is review-before-merge.** It independently reviews every worker
> branch before integrating it (designer≠builder); the merge gate is a **trust obligation, not a
> coordination convenience** — the orchestrator never rubber-stamps a worker's PR to keep the fleet
> moving. The role's *name* foregrounds coordination (spawn/orchestrate the fleet); this clause ensures
> the review duty does not ride on the name.

This is the agreed mitigation for the one gap the rename exposed (`q-1782336693`). It is NOT applied
now (frozen-path, human-only) and rides the same gate as the rest of T38.

## Pointers
ADR-0021 (role resolution; trust-role union vs session-role) · ADR-0017 (writer push tier)
· ADR-0014/0026/0027 (credential adapters — the Level-3 routing seam) · ADR-0019 §5 +
SPEC-playbook `#role` (the D3 drift) · `config/hooks/role-push-guard` (today's hardcoded
2-role git gate) · the 2026-06-24 advisor↔human discussion + confer envelopes
`q-1782327925` / `q-1782328509`.
