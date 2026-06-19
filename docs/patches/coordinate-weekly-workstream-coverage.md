# Patch — add a "Workstream coverage" section to /coordinate weekly mode

**Status: READY (human-apply).** Target is a **trust-path** file
(`.claude/skills/coordinate/SKILL.md`), so the advisor stages this diff; the human
applies it (skill prose = host-apply, per the `.claude` write gate). Ordinary skill
prose — no settings/hook/permission change.

## Why
The human (2026-06-19) asked that the spec-map **workstream overlay**
(`docs/spec-map.md` "Workstream overlay" + `docs/WORKSTREAMS.md` "FR/ADR coverage")
be a recurring part of the weekly report — so cross-stream balance (which streams are
DESIGNED but UNBUILT) is a standing weekly signal, complementing the SPEC-level
Coverage map (§5).

## The edit — `.claude/skills/coordinate/SKILL.md`, "Mode: weekly", the
## "Compose these sections, in order:" list.

INSERT a new section immediately **after §1 Shape** (renumber the rest):

```
2. **Workstream coverage** — the project-level lens (`docs/WORKSTREAMS.md`
   "FR/ADR coverage" + the `docs/spec-map.md` "Workstream overlay"): FRs **and**
   ADRs per stream (The Spine · AI-First · The Run · Target Env · Guardrails ·
   Verification · Dogfood), as a small table with counts. **Flag the gap class
   explicitly**: streams that are DESIGNED (hold ADRs) but UNBUILT (zero FRs) —
   currently The Run + Target Env. This is the cross-stream balance signal; it
   complements the SPEC-section Coverage map (§5, which is FR→test depth) by adding
   the *breadth* view. Numbers come from the registry (`grep -c id: FR-…` by family
   → stream per WORKSTREAMS.md) + the ADR set; if a `/specmap` regen ran this week,
   refresh the overlay counts with it.
```

Then renumber the existing §2–§7 → §3–§8.

Optionally tighten §1 Shape to note the overlay it now carries:
> 1. **Shape** — the spec→FR→thread map: embed/link `docs/spec-map.md`
>    (now carries the **Workstream overlay**; regenerate via `/specmap` first if the
>    registry/threads changed — its own PR).

## Apply
The human edits `.claude/skills/coordinate/SKILL.md` per the above (no `ALLOW_*`
needed — skills are not in the protect-paths deny class, but live under `.claude/`,
so they are host-applied by convention). The next `/coordinate weekly` then emits the
Workstream-coverage section. Re-diff if the weekly section has moved.
