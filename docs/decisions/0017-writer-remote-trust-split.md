# ADR-0017 — Writer remote trust: push-to-branch when unblocked; merge-to-main stays gated
**Date:** 2026-06-12   **Status:** Proposed (authorship chain: **human-decided** — settings-design challenge discussion 2026-06-12 — → **agent-transcribed** — this ADR — → **human-accepted**; acceptance = PR merge, per RULES §5/C3)

## Context
The HIGH-1 permission trim (2026-06-12) moved the six outward allows
(`git push`, `gh pr/api/run/issue/auth`, `WebSearch`) from the shared project
`settings.local.json` into the advisor's user settings. Net effect: loom-author
(Writer) works local-only; loom-advisor is the de-facto git-controller
(push, PR creation, merge clicks per TEAM.md "host-side ops").

The human challenged this shape with two questions this ADR answers:

1. **Is local-only Writer a designed trust boundary or a workaround?** Partly
   workaround: the Writer's `gh` credential/opt-in blocker (envelope 036)
   forced the current arrangement. Left unrecorded, the workaround would
   silently become product shape.
2. **What does the stop actually buy if the controller trusts the Writer's
   work and merges anyway?** Not review quality — containment. The value of
   the gate is RULES §5 mechanism-not-trust: a prompt-injected or runaway
   Writer that cannot reach the remote cannot exfiltrate, rewrite history, or
   publish secrets, regardless of how trusted its average output is. The gate
   also gives a choke point in a different threat context (the merger's
   session was not exposed to whatever the Writer ingested — the T28
   guardrail-drift check happens here) and a single audit/rate-limit point.

That justification is strong for **merge-to-main** (irreversible, publishes)
and weak for **push-to-feature-branch** (reversible, contained by branch
protection). The two should not share one rule.

## Decision
1. **Target state (takes effect when the 036 credential blocker clears):**
   the Writer may `git push` to feature branches and `gh pr create`.
   Allow-list entries scoped accordingly (push to non-main refs, PR create)
   are added to the Writer's seat settings at that time.
2. **Merge-to-main stays gated, permanently.** No agent seat merges its own
   work to main; the merge click belongs to the human or to a reviewing seat
   that did not author the change (TEAM.md self-verdict rule, RULES
   "never self-approve"). Branch protection on `main` is the server-side
   floor — client allow-lists are anti-footgun convenience, not the guardrail.
3. **Today's advisor-as-git-controller arrangement is transitional**, forced
   by 036, and is recorded here as such — not product shape.
4. **No new hat.** "Git-controller" is not a role; push-to-branch folds into
   the Writer seat at unblock time, and merge authority stays with the
   existing review/human gates.

## Alternatives considered
- **B — permanent git-controller hat, Writer local-only forever:** rejected —
  pays the relay bottleneck (advisor pushing Writer's branches) indefinitely
  for no containment gain over Decision 1+2; the dangerous act is the merge,
  not the branch push.
- **C — full Writer push + merge once creds resolve:** rejected — an agent
  merging its own work to main is self-approval of an irreversible act,
  breaking the RULES invariant independent of trust level.
- **Status quo unrecorded:** rejected — workaround-becomes-shape drift; the
  human's challenge is precisely that this was happening silently.

## Consequences
- At 036-unblock time: a settings PR adds Writer push-to-branch + PR-create
  allows; that change is outward-widening, so it is **not** inside the
  pre-authorized stricter-direction envelope — human approval required.
- The advisor sheds relay work (pushing/PR-ing Writer branches) but keeps
  review, arming, and coordinator duties; human keeps frozen-path and
  ADR/phase merges.
- T28 (harness self-defense): merge-gate remains the named drift choke point;
  the guardrail-drift detector wish-list item should diff Writer seat allows
  against this ADR's scope (branch push + PR create, never merge).
- **Prompt-fatigue caveat (human, 2026-06-12, accepted-as-trial):** a
  permission prompt that fires often gets a blind "yes-always" — the gate
  decays into theater and the allow-list grows ungoverned. The design must
  therefore not lean on prompt-reading: deny rules + server-side branch
  protection are the load-bearing layers; prompts must stay rare enough to
  carry signal (a frequently-firing prompt is a design bug — convert it to a
  scoped allow or a mechanism). The T28 guardrail-drift detector is the
  compensating control for "yes-always" allow-list growth.
- Revisit if: branch protection on `main` cannot be enforced server-side; a
  second Writer seat appears (per-seat scoping question); Phase-3 agent
  topology changes who holds review authority; **or the trial shows
  prompt-fatigue in practice** (frequent outward prompts being blind-granted
  — evidence: allow-list growth without matching review acts).
