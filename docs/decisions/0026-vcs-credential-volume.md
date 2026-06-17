# VCS credential volume — sibling `~/.config/gh` volume + gh credential helper — ADR-0026

**Date:** 2026-06-17   **Status: Accepted** (human, 2026-06-17, after adversarial advisor review + gate-green).

This is the **T15-successor credential ADR** that ADR-0015 explicitly deferred. It
formalizes the decision (the volume + helper code lands with it as ONE coherent PR
— advisor-in-loom (T34) Phase 3 Slice B), but it is a **trust/ADR path**: per
ADR-0017 it needs human acceptance, not auto-merge. Provisioning the token is a
one-time human trust act (no headless device flow exists).

> Drafted by an ephemeral loom-author behind the "agent drafts, human accepts"
> pattern (cf. ADR-0015 §harness, ADR-0018, ADR-0023). Spec before code, RULES §2 /
> ADR-0006: this records the decision ADR-0015 left open; the engine/gitconfig
> change is the realization of *this* decision, not drift ahead of it.

## Context — a deferral, not a new question
ADR-0015 carved the harness home into config-that-materializes vs state-that-
accretes, and routed `~/.claude` to a single persistent named volume (T14). It
**explicitly deferred** non-`~/.claude` agent state — naming `~/.config/gh` and VCS
credentials (the T18 push gap) — to a successor ADR:

> *"non-`~/.claude` agent state (e.g. `~/.config/gh`, VCS credentials — T18's push
> gap) needs the volume model widened or a sibling volume; that belongs to the
> T15-successor credential ADR."* — `docs/decisions/0015-harness-home-config-vs-state.md:91-93`

That ADR did not exist (`T15-successor` was a literal string only in ADR-0015,
tracked unstarted in `docs/PLAN.md`, `docs/HARNESS.md`, `docs/TEAM.md`,
`docs/TOPOLOGY.md`). PR #187 added the `gh` **binary** (env-tier tool). What was
still missing: **persistence** so a `gh auth login` token survives `loom build`/
recreate, plus the **credential-helper** wiring so `git push` over HTTPS uses it.
Without those, loom-dev can commit but not push — the advisor's git-controller role
(push / PR / merge) stays blocked. This ADR closes the deferral with the minimal
mechanism.

ADR-0014 found that **env-passthrough leaks** the model token into `Config.Env` /
`docker inspect`. T15's lean has always been **no secret at rest in a loom
artifact**. The credential-helper-on-demand options (a `credential.helper` or
`GIT_ASKPASS` script that fetches a short-lived token) avoid any at-rest token but
add a secret-store dependency and per-use machinery. The volume-backed option keeps
the token at rest, but only in the same trust class as the model-API creds already
accepted under ADR-0015 (T14).

The spike `.scratch/spikes/advisor-gh-cred.md` (H3) probed this empirically inside
loom-dev. Two of its premises were **false against the current tree** (verified
2026-06-17) and are corrected here, not inherited:
- It assumed ADR-0015 *already* routed `~/.config/gh` to the successor volume — it
  does not; ADR-0015 only **defers** it (above). This ADR is what does the routing.
- It assumed the gh credential helper was *already* pre-wired in the global
  gitconfig — it was not; `config/dotfiles/gitconfig` held only `[user]`
  name/email. This ADR adds the helper.

The spike's verdict still stands on its merits: **H3 (`gh auth login`, volume-backed
`~/.config/gh`) is leak-free by construction** — the token is at rest *only* in the
named volume (same trust class as the `~/.claude` model creds), and is **absent from
`loom.yml`, `Config.Env`, `docker inspect`, and the process env** because no `GH_*`
var is declared in the playbook (loom only injects declared env, so the ADR-0014
passthrough leak path is not used for gh). The spike could not exercise push/PR end-
to-end (the token store was unprovisioned and the headless seat is permission-gated)
— that is the human provisioning step below, not a design gap.

## Decision — a sibling persistent volume + gh-native helper, gh only
1. **Sibling volume (ADR-0015's preferred shape, line 92).** A named, per-container
   persistent volume — `<container>-gh` — mounted at `~/.config/gh`, sibling to the
   `<container>-claude` agent-home volume. Hardcoded in the engine beside
   `agentHomeVolume` (there is no `volumes:` config key; the single existing
   persistent volume is hardcoded in Go). Mounted **only when `gh` is a declared
   tool** (`hasTool(spec.Tools, "gh")`) — the parallel of the `~/.claude` mount's
   `hasAgent(..., "claude-code")` gate. An unused empty volume would be harmless, but
   gating on the declared tool keeps the surface honest (no volume for a container
   that has no gh).
2. **Same persistence class as the model creds.** Removed **only** by the opt-in
   `teardown --clean-state` flag, never the `volumes`/`reset` tiers — wiping VCS auth
   must be an explicit choice, exactly as for `~/.claude` (T14). Survives
   `build --force` / recreate (`docker rm` keeps named volumes). No `chown` is added:
   `~/.config/gh` is inside `$HOME`, already covered by the recursive post-home-sync
   ownership fix (it is a writable named volume, unlike the read-only creds bind).
3. **gh-native credential helper in the declared gitconfig.** `config/dotfiles/
   gitconfig` gets the per-host `[credential "https://github.com"]` block that
   `gh auth setup-git` writes — an empty `helper =` to reset any inherited helper,
   then `helper = !gh auth git-credential` (plus the matching `gist.github.com`
   block). Declared config, re-converged each build (in-container edits to
   `~/.gitconfig` are drift and erased), so it must live in the repo, not be set at
   runtime. `git push`/fetch over HTTPS then authenticate via the gh token in the
   volume.
4. **gh only — not a general credential framework.** The token is a **fine-grained
   PAT** scoped to `contents` + `pull_requests` on `iVersatile/loom` only,
   minimizing the at-rest exfil prize. Provisioning it is a **one-time human trust
   act** (`gh auth login` web/device flow, or `gh auth login --with-token` from a
   secret store) — there is no headless device flow, so the agent cannot self-
   provision.

## Alternatives considered
- **On-demand credential helper / `GIT_ASKPASS` (spike H1/H2).** A script that
  fetches a short-lived token per use → **zero token at rest**. Stronger on paper,
  but needs a secret store and per-use fetch machinery loom does not have today, and
  buys little over a volume that is already the same trust class as the accepted
  `~/.claude` creds. **Deferred**, not rejected — revisit if at-rest becomes
  unacceptable (e.g. shared/multi-tenant hosts).
- **Env-passthrough `GH_TOKEN` (ADR-0014 path).** Rejected: ADR-0014 already found
  this leaks the token into `Config.Env` / `docker inspect`. Reusing it for gh would
  reintroduce the exact leak T15 exists to avoid.
- **Host-side push relay (`scripts/push-from-host.sh`, spike H4).** The status quo.
  Rejected for advisor-in-loom — it defeats the purpose of an in-container git-
  controller. Retained only as a fallback.
- **Widen the `~/.claude` volume to cover `~/.config` (one volume).** Rejected:
  conflates agent-harness state with VCS creds, couples their lifecycles, and blurs
  the per-credential trust boundary. A sibling volume keeps each credential class
  independently mountable and revocable — and is ADR-0015's stated preference.

## Consequences
**Positive**
- Closes the ADR-0015 deferral and the T18 push gap with the minimal mechanism;
  unblocks advisor-in-loom Slice B (an in-loom session pushes a branch + opens a PR).
- Leak-free by construction: token at rest only in the named volume, absent from
  `loom.yml` / `Config.Env` / `docker inspect` / process env / committed files
  (`gitleaks` gate stays clean — no token is ever in the repo).
- Mirrors the proven `~/.claude` volume + gate + teardown pattern exactly — one more
  instance of an established shape, not a new subsystem.

**Trade-offs**
- A token *is* at rest in the volume. Accepted as the same trust class as the model-
  API creds already accepted under ADR-0015. **Couples to T20 egress:** the at-rest
  token is the exfil prize — its fine-grained scope (contents + pull_requests on one
  repo) is the mitigation, and egress gating (T20) is the complementary control.
- A separate, non-credential blocker remains: the advisor git-controller seat needs
  a permission-policy allowlist for `git push` + `gh pr create/merge`, or every op
  stalls on approval even once creds exist. Out of scope here (permission stack, not
  the credential model) — flagged so it is not forgotten.

**Revisit if** (inherits ADR-0015:94):
- **Multi-agent homes** (gemini/codex alongside claude) need per-agent credential
  volumes — the `<container>-gh` single-volume shape may need per-agent naming.
- At-rest tokens become unacceptable (shared/multi-tenant hosts) → promote the
  on-demand helper (H1/H2) from deferred to chosen.
- PAT rotation/expiry or GitHub App tokens are needed — **out of scope** here; this
  ADR deliberately does not build PAT rotation, App tokens, per-agent volumes, or a
  general credential framework.

## Supersedes / relationships
Supersedes nothing. **Resolves the ADR-0015 deferral** (the T15-successor credential
ADR it named). Builds on T14 (the `~/.claude` volume pattern), ADR-0014 (why not
env-passthrough), ADR-0017 (this is a trust/ADR path → human acceptance). Couples to
T20 (egress) and the advisor permission-allowlist follow-up.
