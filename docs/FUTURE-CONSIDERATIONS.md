# Future considerations

Parked ideas to revisit later — deliberately **not** yet acted on. This is the
icebox, distinct from:
- `PLAN.md` — the committed phase roadmap (what we *will* do, in order).
- `OPEN-THREADS.md` — design discussions actively being resolved.
- `decisions/` (ADRs) — decisions already made.

When an item is picked up it graduates to one of the above and the entry is marked
✅ with a pointer. Keep entries short; capture enough that the thought isn't lost.

---

## FC-001 — Document the gating pipeline as a durable, drift-proof artifact
- Raised: 2026-06-09 (session discussion)  ·  Status: 🧊 parked

**Idea.** Keep the local-gate → CI → merge gating pipeline (tiers, per-commit vs
boundary, mechanism-not-trust guards) as documentation and/or an artifact linked to
a commit/tag.

**Options considered:**
- *Living doc* (`docs/PIPELINE.md` or a `TESTING.md` section) — versioned, but
  hand-maintained prose rots.
- *ADR* — records the decision/why (stable, append-only).
- *Generated + enforced* — render the checks table from `Makefile` + `ci.yml`; CI
  fails if stale (the ADR-0013 pattern — can't lie).
- *Tag + release notes* — a frozen point-in-time snapshot at phase boundaries.
- *CI attestation/manifest* — audit-grade "what gated this SHA" provenance (heavier).

**Format:** Mermaid (GitHub-rendered, versioned text, exportable to SVG/PNG) beats
ASCII or a committed binary image.

**On-brand recommendation (if picked up):** ADR for the *why* + a living
`docs/PIPELINE.md` (Mermaid narrative) whose mechanical checks-table is **generated
from `Makefile`/`ci.yml` and enforced by an `fr-verify`-style check**, + annotated
phase tags embedding the rendered diagram. git already gives commit-linkage for free
(`git show <sha>:docs/PIPELINE.md`).

**Key caveat:** a hand-drawn pipeline diagram rots into a lie within a few PRs (the
rot-then-lie problem ADR-0013 rejects). Hand-author the narrative/why; **generate and
enforce** the "which checks run where" mapping.

**Lightest first step:** `docs/PIPELINE.md` (Mermaid) + the ADR; the
generator/enforcement reuses the existing `internal/fr` + `make fr-verify` machinery.
