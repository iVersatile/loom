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

## Pointers
ADR-0021 (role resolution; trust-role union vs session-role) · ADR-0017 (writer push tier)
· ADR-0014/0026/0027 (credential adapters — the Level-3 routing seam) · ADR-0019 §5 +
SPEC-playbook `#role` (the D3 drift) · `config/hooks/role-push-guard` (today's hardcoded
2-role git gate) · the 2026-06-24 advisor↔human discussion + confer envelopes
`q-1782327925` / `q-1782328509`.
