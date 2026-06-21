# ADR-0027 — AI-first model-credential acquisition for the agent seat

**Date:** 2026-06-21   **Status: PROPOSED** (advisor draft for human acceptance — T15-successor).

> Drafted by loom-advisor behind the "agent drafts, human accepts" pattern (cf. ADR-0026,
> RULES §5/C3). Spec/ADR-before-code (RULES §2 / ADR-0006). **ON ACCEPT:** move to
> `docs/decisions/0027-ai-first-credential-apikeyhelper.md`, flip Status to **Accepted**,
> commit with `ALLOW_SPEC_CHANGE=1`. Analysis basis:
> `.scratch/spikes/t15-apikeyhelper-credential-acquisition.md`. Live-spike probe:
> `.scratch/spikes/probe-apikeyhelper.sh` (run `FAKE_KEY=1` first — free). **Rev. 2
> (2026-06-21):** reframed around the billing axis the human surfaced — this is no longer
> an apiKeyHelper-by-default decision; it is a choice between two credential models.

## Context — the T15 gap ADR-0014 left open
ADR-0014 chose **interactive in-container OAuth login** — the only path that authenticates
the interactive TUI, but it needs a human + browser. ADR-0005 makes the AI agent a
**first-class user**: it must bring up and operate its env end-to-end. Today it cannot
acquire the model credential without a human ritual (T15). The non-interactive paths
ADR-0014 tested failed the bar: host creds-file is dead on macOS; the env token
(`docker run -e NAME`) **leaks into `Config.Env`/`docker inspect`**. We need a
**non-interactive, leak-conscious** model credential for the **agent seat** (the
interactive *human* seat keeps OAuth login unchanged).

## The decision is a BILLING / IDENTITY choice (not a mechanism detail)
The agent's credential mechanism follows from how you want it to **bill and identify**.
Two viable shapes; **the human picks the axis**:

### Option A — agents on an **API key** (`apiKeyHelper` + secret store)
`harness.claude.apiKeyHelper` declares an operator command (`op read op://…`, vault, …);
loom materializes it into `settings.json` and **never sees/stores/logs the key**. Claude
calls it per request.
- **Billing:** Anthropic **Console / pay-per-token** (`x-api-key`) — **separate from your
  Claude subscription, metered, can surprise-bill under autonomous load.**
- **At rest:** no model key in any loom artifact; but a **store-access token** (e.g.
  `OP_SERVICE_ACCOUNT_TOKEN`) is at rest at the container boundary — a *better trust class*
  (short-lived, scoped, store-owned), not zero.
- **Deps:** a secret store + per-use helper machinery.

### Option B — agents on the **subscription** (minted `CLAUDE_CODE_OAUTH_TOKEN`)
A human runs `claude setup-token` once; loom injects the token at **exec-time** (NOT
`docker run -e`, avoiding the Config.Env leak), persisted in the existing `~/.claude`
volume (ADR-0014/T14 trust class).
- **Billing:** your **Claude subscription** — **flat, no per-token API spend.**
- **At rest:** a **long-lived (≈1-year) token** in the volume — wider blast radius
  (ADR-0014 was wary of this as a *general* default), but the **same trust class as the
  model creds already accepted** there, and mitigated by exec-time injection + rotation.
- **Deps:** none beyond a one-time human `setup-token`.

| Axis | A — `apiKeyHelper` / API key | B — minted subscription token |
|---|---|---|
| Billing | Console, **metered** (surprise-bill risk) | **Subscription, flat** |
| Autonomous (no human per run) | ✓ | ✓ (after one human mint) |
| Secret at rest | store-access token (better trust class) | long-lived OAuth token in volume |
| Interactive TUI | **unknown** (live spike) | likely **headless-only** |
| Machinery | secret-store dependency + helper | none (one-time mint) |
| Precedent | ADR-0014 "revisit if" | ADR-0026 "chose the volume / simple" |

## Decision (recommended, pending the human's billing call)
**Default to B (minted subscription token) for the agent seat; offer A (`apiKeyHelper`) as
an upgrade** where a managed secret store exists AND metered Console billing is acceptable
(e.g. high-isolation / multi-tenant / hosted). Rationale: B keeps **cost predictable**
(no metered surprise under autonomous load), is **simpler** (no store dependency — the
ADR-0026 "choose the volume" logic), and stays in the **already-accepted volume trust
class**; A is the purist "no model key at rest in loom" answer but trades that for metered
billing + a store dependency, and its zero-at-rest claim is **relocation, not elimination**
(the store-access token is still at rest). Either way, loom declares the mechanism; the
key is **the operator's**, never authored by loom.

### Shared invariants (hold for A and B)
1. **Honest framing:** "non-interactive + no `docker inspect` leak," explicitly NOT
   "no secret at rest." Don't overclaim (the FR-006/migrate-spike lesson).
2. **Two seats, two mechanisms:** the interactive **human** seat keeps ADR-0014 OAuth
   login; this ADR is the **agent** seat only. Do NOT unify (ADR-0026 precedent).
3. **Coupled to egress (T20):** an in-container credential is only as safe as the channel
   that can exfiltrate it; name T20 as the companion guardrail; deny-rule the token path
   (mirroring the `Read(~/.claude/.credentials.json)` / `Read(./.env)` denies).
4. **No generalization** beyond the model key (VCS creds stay on the ADR-0026 volume).

## Unknowns — confirm by the live spike before the build freezes
- **A:** does `apiKeyHelper` authenticate the **TUI** or only `claude -p`? (probe
  `FAKE_KEY=1` answers "is it honored" for free; a real key confirms the happy path.)
- **B:** does the minted `CLAUDE_CODE_OAUTH_TOKEN` drive the **TUI** or only headless?
  what is the real token lifetime / rotation story?
- **Either:** is the chosen billing identity acceptable for the agent seat's expected
  volume? (the crux the human surfaced — metered API vs flat subscription.)

## Alternatives considered (rejected)
- **`docker run -e ANTHROPIC_API_KEY`** — the Config.Env / `docker inspect` leak path
  (ADR-0014). Not reconsidered.
- **Volume + interactive login only** (today): human-gated → fails ADR-0005 for agents.
  Stays for the *human* seat.
- **Unify with the ADR-0026 gh volume:** rejected — per-credential-class trust boundaries
  stay independent.

## Consequences
- **Positive:** the agent acquires the model credential **non-interactively**, with no
  `docker inspect` leak; the AI-first premise (ADR-0005) holds. The human chooses the
  cost/identity model explicitly rather than inheriting it by accident.
- **Trade-offs:** A adds a store dependency + **metered billing**; B carries a long-lived
  token (mitigated by trust class + rotation). On **macOS the Keychain is unreachable from
  the Linux container**, so A's store is realistically `op`/vault-with-a-token, not the
  host keychain.
- **Revisit if:** Anthropic ships a first-class non-interactive OAuth/device flow (would
  give B the TUI + subscription billing with no long-lived token — likely the best of
  both), or egress control (T20) changes the in-container-exfil calculus.

## Realization (the build slice this ADR authorizes — AFTER acceptance + the live spike)
- **B:** wire exec-time injection of `CLAUDE_CODE_OAUTH_TOKEN` from the volume (no
  `docker run -e`); a `verify`/`doctor` non-interactive-auth check. Minimal new schema.
- **A:** a `harness.claude.apiKeyHelper` schema field → materialized into `settings.json`
  (mirrors `settings`/`hooks`/`skills`); the Go plumbing never touches the key.
- Both: the `settings.json`/token wiring is **trust-path → propose-as-diff**; the token /
  store-token provisioning is a **human trust act** (like ADR-0026's gh PAT). The Go
  materialization + its FR are autonomously buildable; the credential is never authored
  by loom.
