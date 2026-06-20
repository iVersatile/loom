# WORKSTREAMS — Loom's project-level map

Orient here for **"which arc am I in?"** — one level above the task queue
(`docs/PLAN.md`) and the design threads (`docs/OPEN-THREADS.md`). Every queue row
and thread serves exactly one of these streams. Aliases are human-set (2026-06-19).

**Two kinds of stream:**
- **Feature** — a concrete build surface; you ship code here.
- **Cross-cutting** — a quality the north star imposes on *every* feature; you don't
  ship it standalone, you uphold it everywhere (it still has concrete surfaces).

| # | Alias | Full name | Kind | Charter anchor | Maturity |
| --- | --- | --- | --- | --- | --- |
| 1 | **The Spine** | Environment orchestration | feature | Goal 2 · Phase 2 | spine done; **2nd stack open** |
| 2 | **AI-First** | Agent-autonomy | feature + cross-cutting | North star · Phase 3 | substrate complete + verified |
| 3 | **The Run** | Onboarding & installer | feature | Goal 1 · scenarios 1–2 | **scenario 1 + s2 continuity shipped** |
| 4 | **Target Env** | Portability & topology | feature | scenario 3 · Phases 4–5 | mac validated; rest declared |
| 5 | **Guardrails** | Security integrity | cross-cutting + surface | North star ("worst thing") | strong floors; gaps open |
| 6 | **Verification** | spec→FR→test integrity + observability | cross-cutting | RULES §2 (specs = product) | enforced (gate / fr-verify) |
| 7 | **Dogfood** | Operating model — "Loom is built using Loom" | cross-cutting / governance | Success criterion (dogfood) | live operating model |

---

## 1. The Spine — Environment orchestration  *(feature)*
The product itself: the reconcile engine + **the verbs**. Owns `detect / plan /
build / update` (desired-state reconcile) and `exec / shell` (entry), the
base+stack+overlay playbook, the lockfile, container-per-project isolation, and CI
emission. **This is where the verbs live.** Charter Goal 2; Phase 2 — the **second
stack** (proving the overlay generalizes beyond `go`) is the open frontier.

## 2. AI-First — Agent-autonomy  *(feature + cross-cutting)*
Two faces: **(a)** the product's AI-operability — `--json`, idempotent,
self-describing, auditable on *every* verb (the north-star quality bar over The
Spine); **(b)** the dogfooded autonomous dev-loop — drain / self-wake / ephemeral
fleet / supervising-box (us building Loom). North star; Phase 3. Substrate complete
+ verified (2026-06-19, ADR-0020/0022/0025).

## 3. The Run — Onboarding & installer  *(feature)*
The "one guided run": a menu-driven, situation-detecting entry point; new-machine
comfort (non-technical); established-machine **clean reset with credential
carry-forward, zero loss**. Charter Goal 1; scenarios 1–2. Scenario 1 (`loom
start`, #227) and scenario 2's **continuity half** (`detect --emit-playbook` draft
carry-forward, #229) shipped; the credential-**move** half (`detect --migrate` into
`.env`) remains — security-sensitive, ADR-0014/0026.

## 4. Target Env — Portability & topology  *(feature)*
Where Loom runs: `mac-dev` (validated), `windows-dev` (declared), devcontainer
**import-and-enrich** (Phase 4, never degrade-to), the **cloud sandbox sibling**
(Phase 5 — ephemeral VM, durable state outside it). Charter scenario 3; Phases 4–5.

## 5. Guardrails — Security integrity  *(cross-cutting + surface)*
The north star's *"would the guardrails hold if it tried the worst thing?"*: the
deny-floor, the trust model, the guard hooks, egress control (T20), harness
self-defense (T28), the auditable trail. Wraps every verb and every autonomous action.

## 6. Verification — spec→FR→test integrity + observability  *(cross-cutting)*
The discipline that keeps **"the specs are the product"** honest: the spec→FR→test
joints (`make fr-verify`), the spec-map coverage, the gate, the phase-close review
gates. Every feature stream proves itself here. **Includes observability** — the
audit / reviewable-trail that records what the running system actually did (the north
star's *"every change leaves a reviewable trail"*). The audit trail is **dual-homed**:
Verification owns it as *evidence/review*; **Guardrails** relies on it as an
*accountability mechanism* (a destructive action must be catchable in the trail).

## 7. Dogfood — Operating model ("Loom is built using Loom")  *(cross-cutting / governance)*
The team + process that build Loom **with** Loom: the seats/roles (advisor ↔ author),
the gate + phase-close discipline, AUTOPILOT / trust governance, and the
communication + dev-loop conventions (`RULES.md`, `TEAM.md`). Distinct from its
neighbours: **AI-First** is the autonomy *mechanism*, **Verification** is correctness
*proof* — **Dogfood is *who* does the work and *how* the human↔agent collaboration is
governed.** Charter success criterion: *"Loom is built using Loom, dogfooded from the
first runnable slice."*

---

## Where common things belong
- **The verbs** (`build/exec/shell/detect/plan/update`) → **The Spine** (the product
  interface) — with **AI-First** (`--json`/idempotent) and **Guardrails** (deny/audit)
  as quality wrappers over each verb.
- **Credential carry-forward** → **The Run** (scenario-2 reset) + **Target Env**
  (cloud durable-state-outside-the-VM).
- **Audit / reviewable trail** → **Guardrails** + **Verification**.
- **Playbook / lockfile / reconcile** → **The Spine**.

## FR / ADR → workstream coverage (2026-06-19)
The mapping is **latent** — FR family prefixes already cluster by concern — so this
is a lens over the existing registry, not new metadata. (A queryable per-FR
`workstream:` field is a deferred option; this index is the lightweight version.)

**FRs by workstream** (75 total; counts current to #229):
| Workstream | FR families | ~FRs |
| --- | --- | --- |
| The Spine | BUILD 16 · SCHEMA 11 · EXEC 4 · ENTRY 4 · TEARDOWN 4 · PLAN 3 · DETECT 2 · SHELL 1 | **45** (60%) |
| AI-First | LOOP 4 · INV 5 (the `--json`/idempotent/auditable invariants) | 9 |
| Guardrails | GUARD 9 (the 3 `role:`/`user:` SCHEMA FRs also lean here) | 9 |
| Verification | DOCTOR 6 (the verb lives in The Spine; *serves* verification) · LOG 2 (observability) | 8 |
| **The Run** | RUN 6 (start scenario-1 · 1–4; detect `--emit-playbook` continuity · 5–6) | **6** |
| **Target Env** | — (only touched by BUILD/SCHEMA) | **0 dedicated** ⚠ |
| **Dogfood** | — (governed by RULES/TEAM, not FRs by nature) | **0** |

**ADRs by workstream** (25 total; listed by *primary* — several are multi-stream):
| Workstream | ADRs |
| --- | --- |
| The Spine | 0001 isolation · 0002 playbook+lockfile · 0004 two-tier config · 0006 engine/policy · 0008 Go · 0011 sentinel · 0012 base image · 0015 harness home · 0016 entry verbs |
| AI-First | 0005 AI-first user · 0020 closed-loop · 0022 substrate · 0025 fleet/supervisor |
| Guardrails | 0017 remote trust · 0018 trust flags · 0019 non-root · 0021 role resolution · 0023 edit-guard |
| Target Env | 0003 devcontainer import · 0007 cloud sandbox |
| The Run | 0014 in-container cred login · 0026 VCS cred volume |
| Verification | 0010 audit+diag logs · 0013 FR registry |
| Dogfood | 0009 dogfood-stack Go |
*(multi-stream examples: 0016 also Verification (audit); 0017/0021/0023 also Dogfood; 0019 also The Spine; 0014/0026 also AI-First git autonomy.)*

**The coverage signal — sharper than prose:**
- **The Spine holds ~60% of all FRs** — the requirement mass *is* the product engine.
- **The Run now holds 6 FRs** — scenario 1 (`loom start`, #227) + scenario 2's
  continuity half (`detect --emit-playbook`, #229); the `--migrate` cred-move half
  remains. **Target Env still has ADRs (decided) but ZERO FRs (unbuilt)** — *designed,
  not implemented*. That is now the precise remaining gap.
- **Dogfood = 1 ADR + 0 FRs** — expected: it lives in RULES/TEAM as process.

## Resolved (human, 2026-06-19)
- **Operating Model / Dogfood → promoted to stream #7** (governance is a distinct axis).
- **Observability → folded into Verification (#6)**, cross-referenced from Guardrails
  (the audit trail is dual-homed: evidence + accountability).

## Honest status signal (2026-06-19)
The board is concentrated in **AI-First** (its harness face). **The Spine** (second
stack) and **The Run** (installer) — two of the three charter goals — are
comparatively under-served. Per RULES §5 / TEAM.md p1: *the product is the specs
made real; harness work exists to serve that.* A deliberate re-balance toward
The Spine / The Run is worth weighing when choosing the next workstream.
