# ADR-0027 — Per-project credential convention (all agents + gh); resolver deferred

**Date:** 2026-06-21   **Status: PROPOSED** (advisor draft for human acceptance — T15-successor).

> "Agent drafts, human accepts" (RULES §5/C3). **ON ACCEPT:** move to `docs/decisions/0027-…`, flip to
> **Accepted**, commit with `ALLOW_SPEC_CHANGE=1`. Confer + analysis:
> `.scratch/spikes/t15-credential-framework-shape.md` (+ `…/t15-per-project-credentials.md`,
> `…/t15-apikeyhelper-credential-acquisition.md`); probe `…/probe-apikeyhelper.sh`.
> **Rev 4 (2026-06-21):** after a 3-agent confer on a *unified framework* — re-aimed from "claude
> model creds" to a **cross-agent credential CONVENTION**, with the honest finding that the
> **resolver/framework is premature** (the deciding example, Gemini ADC, isn't built) so we commit the
> **convention** and **defer the resolver**.

## Context
loom is container-per-project on one host; projects may use different billing identities; it supports
multiple agent harnesses (claude, codex/OpenAI, gemini) + gh for VCS. ADR-0014/0026/0027(rev1-3)
solved credentials **per class** (gh = volume+helper; claude = apiKeyHelper/token). The question "why
is gh managed differently from claude?" is fair: going forward it shouldn't be. **But the confer found
the *unifying* asset is the INVARIANTS + a declaration grammar, not a resolver** — and that the two
built examples (claude, gh) are the two MOST ALIKE (both store-volume, container-internal), so a
universal resolver extracted now would be guessed from the wrong examples.

## Decision — commit the CONVENTION; defer the resolver
Credential acquisition is a **per-project, per-agent/service, pluggable declaration**, governed by one
**cross-class convention**. loom assumes no default and never authors/sees/logs the secret; the
container-per-project boundary isolates one project's credential from another's.

### 1. The shared convention (the real "framework")
- **Per-agent declaration:** `harness.<agent>.credential` (a scalar; merged any-tier last-non-empty-wins
  like `trust`, NOT through the base-gated `settings`). gh is described by the convention but keeps its
  ADR-0026 mechanism (see §3). gemini/codex plug in as declarations, not new mechanisms.
- **Two source shapes:** **S1 value-resolver** (returns a secret string — model API keys, helper stdout,
  the M1 token) and **S2 store-volume** (loom provisions a persistent credential store/file the consumer
  reads + refreshes — gh's `~/.config/gh`, Gemini OAuth/ADC, claude's `.credentials.json`).
- **A fixed consumer adapter ENUM** (no open-ended resolver): `env` · `apiKeyHelper` (stdout-helper) ·
  `volume-token` · `volume-store+helper` (gh) · `oauth-file` (Gemini ADC) · `interactive-login`
  (the human-seat fallback for every service).
- **The 7 INVARIANTS are a shared cross-class rule, MECHANICALLY CHECKED** (asserted ≠ guarded):
  per-project namespacing + **slug-uniqueness (doctor-checked)** · **N store tokens each scoped to one
  project, no shared (doctor-checked: detect cross-project token reuse, fail)** · fail-closed (no source
  ⇒ zero creds, never inherited) · metered spend cap out-of-agent + a `doctor` "which billing identity?"
  check (no secret) · container-internal / host-OS-agnostic (host keyrings unreachable in-container; no
  host path in any trust boundary) · **T20 egress = BLOCKING dependency** · loom never holds the key;
  honest framing ("no leak" ≠ "no secret at rest").

### 2. claude is the first concrete adapter
M1 (`volume-token` → `CLAUDE_CODE_OAUTH_TOKEN`, exec-time, flat subscription) and M2 (`apiKeyHelper` →
settings.json, metered) — per-project, the operator owns the key. (Interactive human seat keeps
ADR-0014 OAuth login.)

### 3. gh = WRAP, not migrate
gh keeps its ADR-0026 mechanism (`<container>-gh` volume + `!gh auth git-credential`). The convention
**describes** it (`method: volume-store+helper`); it does **NOT** re-plumb `ghConfigVolume` or the
gitconfig helper. Rationale: ADR-0026 is frozen + deliberately anti-framework + keeps per-class trust
boundaries independent for independent revocation; a shared resolver call-site would re-couple exactly
that. **This ADR extends/references ADR-0026; it does not supersede it.**

### 4. DEFERRED — the universal resolver / named-profile selector (was M4)
A `credential.source` named-profile → backend resolver table is **deferred until Gemini lands**. Gemini's
ADC/OAuth file+refresh is the third, shape-deciding example that determines whether S2 needs a first-class
`oauth-file` contract or just a volume. Building the resolver now guesses that contract from the two
most-similar classes. A unified resolver is also **blast-radius-increasing** (a config surface for a
host-wide token) — earn it with evidence; the convention does NOT add a shared trust root.

## Unknowns / live-rechecks (before any freeze)
claude `apiKeyHelper` TUI-vs-headless (`probe`, `FAKE_KEY=1` free) + precedence order; Gemini CLI env
names + ADC paths + helper-hook; Codex `~/.codex/auth.json` + project config. The Gemini ADC spike (does
its OAuth file survive a per-project volume + refresh?) is the gate that turns the resolver from
"premature" into "extractable."

## Alternatives considered (rejected)
Build the full resolver framework now (premature — guessed from the 2 most-alike examples; ADR-0026 +
0027-M4 both already deferred it); migrate gh into a unified store (re-couples ADR-0026's independent
boundaries for no new capability); one global identity for the host (wrong axis for container-per-project);
`docker run -e SECRET` (the Config.Env leak).

## Consequences
- **Positive:** every agent + gh is governed by one **convention** (uniform declaration grammar +
  invariants) — each new agent is a plug-in declaration, answering "why is gh different?" (going forward
  it isn't); independence + isolation are *mechanically checked*, not assumed; no premature framework.
- **Trade-offs:** the resolver convenience waits for Gemini; gh remains its own (wrapped) mechanism for
  now; metered M2 carries the store-token + billing caveats.
- **Revisit / next:** when Gemini lands, spike its ADC against a per-project volume → decide the
  `oauth-file` contract → THEN extract the resolver (the rule-of-three third instance).

## Realization (the build this authorizes — AFTER T20, ADR acceptance, the live spike)
First slice = **the convention + the claude adapter**: a `HarnessAgent.Credential{Method,Helper,Env}`
field (any-tier merge); a per-adapter materialize dispatch (settings.json for M2 via key-merge;
exec-time volume-token injection for M1 — the one genuinely new path); `serviceVolume(container,svc)`
generalizing the hardcoded `-claude`/`-gh` pair; `doctor`/`verify` checks (slug-uniqueness +
cross-project token reuse + resolved-billing-identity). The Go plumbing + FRs are autonomous; the
credential is never authored by loom; the settings/token wiring is trust-path → propose-as-diff; the
per-project key/token provisioning is a human trust act. The resolver is NOT in this slice.
