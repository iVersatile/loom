# ADR-0027 — AI-first model-credential acquisition via a pluggable `apiKeyHelper`

**Date:** 2026-06-21   **Status: PROPOSED** (advisor draft for human acceptance — T15-successor).

> Drafted by loom-advisor behind the "agent drafts, human accepts" pattern (cf. ADR-0026,
> ADR-0015 §harness, RULES §5/C3). Spec/ADR-before-code (RULES §2 / ADR-0006).
> **ON ACCEPT:** move to `docs/decisions/0027-ai-first-credential-apikeyhelper.md`, flip
> Status to **Accepted**, and commit with `ALLOW_SPEC_CHANGE=1` (frozen-contract). Analysis
> basis: `.scratch/spikes/t15-apikeyhelper-credential-acquisition.md`. **Two empirical
> unknowns (below) should be confirmed by a live spike BEFORE the build slice freezes** —
> the policy decision stands either way; the build is gated on them.

## Context — the T15 gap ADR-0014 left open
ADR-0014 chose **interactive in-container OAuth login** as the credential path that
authenticates the interactive TUI — but it requires a human with a browser. ADR-0005
makes the AI agent a **first-class user**: an autonomous agent must bring up and operate
its own env end-to-end. Today it cannot acquire the model credential without a human
ritual (T15). The two non-interactive paths ADR-0014 tested both failed the bar: the
host creds-file is dead on macOS (Keychain, no file), and the env token
(`docker run -e NAME`) **leaks the value into `Config.Env`/`docker inspect`** — which is
why API keys are kept OUT of the default `env:`. ADR-0014's own "revisit if" names the
fix: **a secret-store integration (`apiKeyHelper`)**. ADR-0015 deferred non-`~/.claude`
credential state to "the T15-successor credential ADR"; ADR-0026 was that successor *for
VCS creds* (a sibling volume + helper) and explicitly **did not generalize**. This ADR
is the T15-successor *for the model (Anthropic) credential*.

## Decision
**Adopt a pluggable `apiKeyHelper` as the AI-first model-credential path for autonomous /
headless seats.** The playbook declares `harness.claude.apiKeyHelper` — an **operator-
provided command** — which loom materializes into the agent's `settings.json`. Claude
Code invokes it per request (on its TTL) to obtain the key from whatever secret store the
operator wires (`op read`, a vault CLI, etc.). **loom never sees, stores, logs, or
transmits the key**; it authors only the *wiring* (the command string).

1. **Pluggable, not store-specific.** loom hardcodes no secret store. The operator's
   command is the seam (composes with the `op://` reference convention from `detect
   --migrate`). loom validates/materializes the declaration; the key path is the
   operator's.
2. **Honest framing — relocation, not elimination.** This removes *the model key at rest
   in a loom artifact* and the `docker inspect` leak. It does **not** achieve "no secret
   at rest": the helper needs a **store-access credential** (e.g. an
   `OP_SERVICE_ACCOUNT_TOKEN`) at rest at the container boundary. The win is a **better
   trust class** — the store-access secret can be short-lived, narrowly scoped to one
   item, and revocable independently — not zero secrets. The ADR/docs MUST state this
   plainly (no overclaim).
3. **Two seats, two mechanisms — do NOT unify.** The interactive **human** seat keeps the
   ADR-0014 OAuth + `~/.claude` volume path unchanged. `apiKeyHelper` is the **agent**
   path. This mirrors ADR-0026's per-credential-class separation (sibling, not unified).
4. **Coupled to egress (T20).** An in-container key is only as safe as the channel that
   can exfiltrate it. The store-access token is now the prize. This ADR names **egress
   control (T20)** as the companion guardrail and constrains the store-access credential
   to **single-item scope + short TTL**; a deny-rule on its path mirrors the existing
   `Read(~/.claude/.credentials.json)` / `Read(./.env)` denies in `settings.json`.
5. **No generalization** beyond the model key (ADR-0026 precedent). VCS creds stay on the
   volume; this is the model-key path only.

## ⚠ Two unknowns — confirm by a live (docker, human-run) spike before the build freezes
1. **TUI vs headless.** Does `apiKeyHelper` authenticate the **interactive TUI**, or only
   `claude -p`? (The env token authenticated `-p` but not the TUI — ADR-0014.) If
   headless-only, this still solves the autonomous-worker case but the interactive seat
   keeps OAuth. The build's FR must assert whichever holds.
2. **Billing/identity.** `apiKeyHelper` supplies an **Anthropic API key** (console,
   pay-per-token, `x-api-key`), not the OAuth **subscription** token loom-dev uses today.
   Adopting it switches the billing/identity model for the seat that uses it — a product
   choice the human owns before the build.

## Alternatives considered
- **Minted long-lived `CLAUDE_CODE_OAUTH_TOKEN` in a volume** (T15 option 2): a
  `claude setup-token` once, injected at exec-time (not `docker run -e`, avoiding the
  Config.Env leak). Simpler, no store dependency — but a **year-long token = wide blast
  radius** (ADR-0014 rejected as default) and still a token at rest. **Kept as the
  pragmatic fallback** when no secret store is available, not the default.
- **Creds volume only** (ADR-0014 addendum / T14): the human login made durable. Reduces
  ritual frequency but **does not eliminate the human** → fails ADR-0005 for agents. This
  is today's state; it stays for the human seat, not the agent.
- **`docker run -e ANTHROPIC_API_KEY`**: the rejected leak path (Config.Env / docker
  inspect). Not reconsidered.
- **Unify with the ADR-0026 gh volume**: rejected — ADR-0026 deliberately kept per-cred-
  class trust boundaries independent; the model key's autonomous/leak-free requirement is
  a *different* problem than gh's push gap.

## Consequences
- **Positive:** an autonomous agent acquires the model key **non-interactively** with **no
  model key at rest in any loom artifact** and **no `docker inspect` leak**; the AI-first
  premise (ADR-0005) holds at step one; the store-access secret is a better trust class.
- **Trade-offs:** a **secret-store dependency** + per-use machinery (the cost ADR-0026
  avoided for gh by choosing the volume); the store-access token is the new in-container
  prize (mitigated by scope/TTL + T20 egress); on **macOS the Keychain is unreachable from
  the Linux container**, so the store is realistically `op`/vault-with-a-token, not the
  host keychain; the **billing model shifts** to API-key for the agent seat (Unknown 2).
- **Revisit if:** Claude Code adds a first-class non-interactive OAuth/device flow (would
  remove the API-key-billing trade-off), or egress control (T20) lands and changes the
  in-container-exfil calculus, or a host keychain becomes container-reachable.

## Realization (the build slice this ADR authorizes — AFTER acceptance + the live spike)
- Engine: a `harness.claude.apiKeyHelper` schema field (sibling to `settings`/`hooks`/
  `skills`) → materialized into `settings.json`; a deterministic, testable materialization
  path + an FR (the Go plumbing never touches the key). Mirrors the existing harness
  wiring; autonomously buildable.
- Trust-path: the `settings.json` change + any companion helper script are trust-path →
  **propose-as-diff for host-apply**. The store-access token provisioning is a **human
  trust act** (like ADR-0026's gh PAT). `verify`/`doctor` gains a non-interactive-auth
  check (the FR home T15 names).
