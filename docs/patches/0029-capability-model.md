# PATCH PROPOSAL — ADR-0029 (capability model) + ADR-0017/0021 amendments

> **Staged for human `ALLOW_SPEC_CHANGE`** (frozen contracts — `is_frozen()` matches all
> `docs/decisions/*.md`). Advisor-authored from the ratified T38 design (PR #293). To apply:
> `cp` the ADR-0029 body below to `docs/decisions/0029-role-capability-model.md`, apply the two
> amendment blocks to the named ADRs, then `ALLOW_SPEC_CHANGE=1 git commit`. Acceptance = merge.
> Nothing here is built; the de-hardcode/rename/D3 mechanism follows (batch `adv-role-rename-batch`).

---

## NEW FILE — `docs/decisions/0029-role-capability-model.md`

# ADR-0029 — Role-capability model: the closed control-point set

- **Status:** Proposed (acceptance = merge, RULES §3). **Date:** 2026-06-25.
- **Threads:** T38. **Builds on:** ADR-0021 (identity / union / narrow). **Amends:** ADR-0017
  (writer trust → tier map). **Couples:** ADR-0014/0026/0027 (credential routing = the Bundle seam),
  ADR-0028 (egress = an Envelope boundary), ADR-0025 (the orchestrator = persistent supervisor;
  review-before-merge is its defining duty). **Supersedes-in-part:** the two hardcoded roles in
  `config/hooks/role-push-guard`.

## Context
loom hardcodes exactly two roles (`loom-author`, `loom-advisor`) across ~7 mechanism files. A human
scenario (project-ABC: planner/dev/supervisor + DB-owner/data-manager, a staging→PROD pipeline) broke
that two-role assumption and asked whether richer, project-defined roles can resolve. Two prior
attempts to enumerate loom's "capability domains" mixed abstraction levels (e.g. `shell/exec` is not a
sibling of `git` — it is the universal lever; credentials read as "dual-homed"; egress as
"resource-here / enforced-there"). Those were symptoms of an unnamed sorting axis.

## Decision
**1. A loom role is a generic, forge-proof IDENTITY → a policy across a CLOSED capability set.** loom
prescribes as little as possible about what an identity *does*; boundaries are applied by the layer
that owns each, keyed off the identity. (Rejected: a general per-project capability/policy engine —
that is a weaker, bypassable copy of authz a downstream system already owns; *guardrails are
mechanism, not trust*.)

**2. The capability set is sorted by ONE axis — the CONTROL POINT in an action's lifecycle — with a
DERIVED enforcer (is there a downstream owner at that point?). Exactly four buckets:**

| Bucket | Control point | Acts on | Enforcer |
|---|---|---|---|
| **Envelope** | **create**-time | the container (network posture, mounts, cred-volume attach) | **loom** (no downstream) |
| **Gates** | **invoke**-time | a command (git push/merge tier, shell/exec limits, spawn authority) | **loom** (the guards) |
| **Bundle** | **use**-time | a downstream system (scoped DB/cloud/forge keys) | **downstream** (DB GRANT, cloud IAM, branch protection) |
| **Floor** | **always** | loom itself (audit, trust files, role marker, agent-fabric) | **loom**, role-invariant |

This collapses the three criteria the old taxonomy tangled (who-owns / which-mechanism / where-the-
resource-lives) into one. `shell/exec` is not a bucket — it gates *invocations* (Gates); the *envelope*
(Envelope, set once at create) is what bounds standing reach, reachable by any in-container process.

**3. The bright line (charter-level guardrail):**
> loom **self-enforces at create + invoke**; at **use** it only routes a scoped credential and the
> **downstream system enforces**; the **floor** is constant and role-invariant. loom enforces a use-time
> (Bundle) boundary itself ONLY where **no downstream enforcer exists** — today that residual is exactly
> **egress** (deny-by-default network: there is no downstream owner of "this container may not reach the
> internet"). Where a downstream enforcer exists (DB GRANT, cloud IAM, forge branch-protection), loom
> NEVER reimplements that domain's authz.

**4. The closed-set property (the non-negotiable that keeps this a deepening, not an IAM platform):**
the kernel is a **CLOSED set** — exactly these four buckets. Projects parameterize **identities** onto
existing dials; they NEVER declare new control mechanisms. Closed-set kernel = a deepening of a dev-env
tool; open/pluggable = the charter non-goal.

**5. Role design.** A role = `{gate-tiers} + {credential needs} + {envelope needs}` — small by
construction. Heavy authority (DB schema-vs-data, branch-scoped merge) lives downstream or in the forge,
never in the role. Gates are **per-session**; Envelope + Bundle are **per-container** (the union marker,
ADR-0021): a container is built for the *union* of the roles it may host; each session narrows to one;
Gates enforce per-command (narrow-within, never widen). Floor holds for all.

**6. Native roles** (the two loom ships) are `worker` (the fail-closed floor; ephemeral; push-no-merge)
and `orchestrator` (elevated; persistent; merge + may-spawn) — the first two rows of the signed
`role→git-tier` map. The orchestrator's **defining duty is review-before-merge** (see ADR-0025).

## Consequences
- **De-hardcode** the two roles into a flat, signed, fail-closed `role→git-tier` map (ADR-0017 amend);
  unknown role ⇒ no-push floor; the map is a protect-path; the lookup is an exact name→tier-enum match,
  never a policy parse (bash-guard brittleness makes any richer parse a fail-open bug farm).
- **D3 shrinks** to "valid role = a name present in the signed tier-map" (fail-closed otherwise).
- **Egress** is the one self-enforced use-residual; the spawn-rate ledger STRICT mode stays gated on the
  D1 append-but-not-rewritable decision.
- **Projects add roles** (planner/dev/supervisor, DB-owner/data-manager) as more identities onto the same
  closed dials + credential routing; loom learns no new mechanism.

---

## AMENDMENT — `docs/decisions/0017-writer-remote-trust-split.md`
Append to the Decision/Consequences: the two-tier writer-trust split (Decision 1, FR-GUARD-PUSH) is
**generalized into a signed `role→git-tier` map** (ADR-0029 Gates bucket). `loom-orchestrator → merge`,
`loom-worker → push-no-merge`, unknown → no-push floor. Conditions (binding): the map is a protect-path
(ALLOW_TRUST_CHANGE); the marker stays root-owned/unforgeable (ADR-0021); unknown role fails CLOSED to the
floor; the lookup is a dead-simple exact name→tier-enum match, never a policy-language parse. The current
`loom-author`/`loom-advisor` tiers become two rows under their renamed identities (`worker`/`orchestrator`).

## AMENDMENT — `docs/decisions/0021-role-resolution-multi-role-per-container.md`
Append a reference: the trust-role marker's union resolves to capability via ADR-0029's four buckets —
**Gates are per-session** (keyed by the narrowed session-role at invoke time), while **Envelope + Bundle
are per-container** (built once from the union of hosted roles at create time). "Narrow-within, never
widen" is the Gates invariant; the Floor is role-invariant.
