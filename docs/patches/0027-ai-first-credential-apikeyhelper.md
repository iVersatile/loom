# ADR-0027 — Per-project, pluggable model-credential sources (no assumed default)

**Date:** 2026-06-21   **Status: PROPOSED** (advisor draft for human acceptance — T15-successor).

> Drafted by loom-advisor ("agent drafts, human accepts", RULES §5/C3). **ON ACCEPT:** move to
> `docs/decisions/0027-…`, flip to **Accepted**, commit with `ALLOW_SPEC_CHANGE=1`. Analysis +
> confer (design-precedent / loom-fit / red-team): `.scratch/spikes/t15-per-project-credentials.md`
> (and `…/t15-apikeyhelper-credential-acquisition.md`). Live-spike probe:
> `.scratch/spikes/probe-apikeyhelper.sh` (`FAKE_KEY=1` = free).
> **Rev 3 (2026-06-21):** REFRAMED from "one billing model for the agent seat" to **per-project,
> pluggable, no-default** — loom is container-per-project on one host; different projects may use
> different billing identities; loom assumes none.

## Context
ADR-0014 made interactive OAuth login the credential path, but it needs a human + browser; the
env-token path leaks (`Config.Env`/`docker inspect`). ADR-0005 needs an autonomous agent to acquire
the model credential non-interactively. The earlier framing of this ADR (rev1/2) treated that as ONE
global choice. **It is not:** loom runs **container-per-project on one host**, and different projects
may bill to **different identities** (a flat subscription, or different metered API keys per
client/team), on **Mac or Windows**. So the credential source is **per-project playbook data**, with
**no default loom assumes**.

## The boundary already exists
Per-project credential isolation is already built: container per project (`<project>-dev`, labeled
`loom.project`), the agent-home volume is **per-container** (`<container>-claude`) so one project's
creds are unreachable from another's container, the materialize→`docker cp`→per-project-volume chain
is per-project, and loom injects **only declared env**. The redesign adds a per-project
**declaration** + **no default** + **stated invariants** — not a new isolation mechanism.

## Decision
**Credential acquisition is a per-project, pluggable declaration. loom assumes no default and never
authors/sees/logs the secret; the container-per-project boundary isolates one project's credential
from another's.** A project's playbook declares which METHOD its container uses; loom materializes
that method's wiring into that project's container only.

### Methods a project may declare (loom supports the mechanism; the operator owns the key)
- **M1 — per-project subscription token (DEFAULT-capable, not assumed):** `claude setup-token` →
  `CLAUDE_CODE_OAUTH_TOKEN` persisted in the per-project `<container>-claude` volume, injected at
  exec-time (never `docker run -e`). **Flat subscription billing**, simplest, in the already-accepted
  volume trust class. Long-lived token at rest in that project's volume; likely headless-only.
- **M2 — per-project `apiKeyHelper` (the metered/multi-tenant upgrade):** `harness.claude.apiKeyHelper`
  = an operator command (`op read op://client/…`, vault, …) materialized into that project's
  `settings.json`; Claude resolves the key on demand. **Console/metered billing.** No model key at
  rest in a loom artifact, but a **per-project-scoped store-access token** is at rest at the boundary
  (relocation, not elimination). TUI support unconfirmed (live spike).
- **M3 — host-resolved + exec-time injection (niche):** loom runs the operator's resolver on the HOST
  (where the OS keyring IS reachable) and injects per-project at exec-time. For operators who want the
  host keyring as root-of-trust; loom touches the secret in transit.
- **M4 — general `credential.source` selector (DEFERRED):** named-profile → resolver table driving
  Anthropic + gh + registry together. Defer until a 2nd credential class pulls it (ADR-0026
  "don't build a framework early").

**Recommendation:** ship **M1 as the per-project default + M2 as the declared upgrade** (this is the
prior A/B, *parameterized per project*). M3 niche, M4 deferred. The interactive **human** seat keeps
ADR-0014 OAuth login (two seats; do not unify — ADR-0026 precedent).

## Invariants (MUST hold — isolation is stated, not incidental)
1. **Per-project namespacing is an invariant.** Every credential artifact (volume, helper output,
   settings.json, store token) is namespaced to one `loom.project=<name>`; unreachable from a
   container with a different label. **Validate project-slug uniqueness** before it keys a volume.
2. **No shared store token (the M2 trap).** N projects → **N store tokens, each store-side-scoped to
   read only that project's secret.** One shared token = a master credential → forbidden.
3. **Fail-closed.** No declared source ⇒ **zero** credentials + a loud failure — never ambient host
   creds, never a sibling project's. No default, no fallback chain. Never reintroduce `-e SECRET`.
4. **Metered spend cap out-of-agent.** Any M2 credential carries a per-project spend cap enforced
   outside the agent (store budget / Console limit); `doctor`/`verify` names the resolved billing
   identity without printing the secret (catch mis-attribution before spend).
5. **Container-internal, host-OS-agnostic mechanism.** Host OS keyrings (Keychain / Credential Manager
   / Secret Service) are unreachable from a Linux container — **no host filesystem path may be in any
   credential's trust boundary** (kills the Keychain-unreachable AND WSL2-path-sharing failures).
   Normalize CRLF in token files.
6. **T20 (egress) is a BLOCKING dependency.** Every blast-radius bound is void if a compromised agent
   can exfiltrate the token. No Docker socket in a project container.
7. **loom never authors/sees/logs the secret; honest framing** ("non-interactive + no `docker inspect`
   leak," NOT "no secret at rest"). Per-credential-class boundaries stay independent (ADR-0026).

## Unknowns — live spike before the build freezes
M2 `apiKeyHelper` TUI-vs-headless (`FAKE_KEY=1` is a free mechanism check); M1 token TUI/lifetime; and
the per-project billing identity is acceptable for that project's volume (metered vs flat).

## Alternatives considered (rejected)
`docker run -e SECRET` (the Config.Env/`docker inspect` leak — ADR-0014); volume + interactive login
only (human-gated — fails ADR-0005 for agents; stays for the human seat); one global identity for the
host (the rev1/2 framing — wrong axis for container-per-project); unify all credential classes under
one token (ADR-0026 per-class independence).

## Consequences
- **Positive:** projects on one host run on **different billing identities**, isolated by construction;
  no global default is ever assumed (an undeclared project resolves nothing); cross-platform clean
  (container-internal mechanism); the cost/identity model is explicit per project, not inherited.
- **Trade-offs:** M2 adds a store dependency + metered billing + a per-project store token to provision;
  M1 carries a long-lived token per volume. The operator provisions each project's key (a human trust
  act, per ADR-0026's PAT model).
- **Revisit if:** Anthropic ships a non-interactive OAuth/device flow (gives M1 the TUI + subscription
  with no long-lived token); T20 lands (changes the exfil calculus); a 2nd credential class pulls M4.

## Realization (the build slice this authorizes — AFTER acceptance + the live spike)
- Schema: a per-project `harness.<agent>.credential` (and/or `apiKeyHelper`) scalar, merged **any-tier
  last-non-empty-wins like `trust`** — NOT through the `settings` base-tier gate (`load.go:81-91`).
- Engine: a materialize step writing the declared method into that project's container (settings.json
  `apiKeyHelper` for M2; exec-time token injection from the per-project volume for M1); project-slug
  uniqueness validation; a `doctor` "which billing identity does this project resolve?" check (no
  secret). The Go plumbing + its FR are autonomous; the credential is never authored by loom; the
  settings/token wiring is trust-path → propose-as-diff; the per-project key/token provisioning is a
  human trust act.
