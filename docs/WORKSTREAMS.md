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
| 3 | **The Run** | Onboarding & installer | feature | Goal 1 · scenarios 1–2 | **barely started** |
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
carry-forward, zero loss**. Charter Goal 1; scenarios 1–2. Barely started.

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
