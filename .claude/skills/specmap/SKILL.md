---
name: specmap
description: Regenerate docs/spec-map.md — the Mermaid map of SPEC sections, FR coverage, and threads shadow-overlaid on both. Use when the user asks /specmap, "regenerate the spec map", or wants the current spec→FR→thread shape after registry or thread changes. Writes via the normal branch+PR flow (unlike /achievements).
---

# /specmap — regenerate the spec/FR/thread map

Sibling of `/achievements` (that one narrates the queue; this one draws the
contracts). The map joins three layers: **SPEC sections** (the contracts),
**FR coverage** (the registry's `source:` joints + `tests:`), and **threads**
(the shadow layer from OPEN-THREADS). Mechanical gathering is the helper's
job; the **joins, edge classification, and narrative are yours** — edges are
judgment calls, reviewed not parsed (the map's own header says so).

## Invocation

`/specmap` — no arguments; always a full regeneration of `docs/spec-map.md`.

## Procedure

1. **Gather** (read-only): `sh .claude/skills/specmap/gather.sh` emits
   (a) FR-registry entries — id, `source:` cites, tests-present, status;
   (b) OPEN-THREADS headings + status markers + their `Pointers:` lines;
   (c) SPEC-verbs / SPEC-playbook section headings.
2. **Join FRs → spec sections:** the `source:` cite is the joint
   (`SPEC-verbs.md#exec` → node `exec`). Count active FRs per section and
   whether each has tests. Node class: `done` (FRs present & covered) ·
   `inprog` (spec live, FRs/implementation pending) · `untouched` (spec'd,
   nothing built).
3. **Join threads → sections:** start from each thread's `Pointers:` line,
   then apply judgment for unlisted relationships. Shadow nodes carry the
   thread id + status marker (✅ resolved · 🟢 decided/in-build · 🟡 open).
4. **Classify edges:** dotted = overlap / part-of; labeled `✕` = blocker /
   precursor. No edge = no spec relationship. When unsure, prefer no edge
   over a speculative one — the map is read as canon-shaped.
5. **Render** in the existing `docs/spec-map.md` format (compare against the
   current file before overwriting): legend block, one `flowchart LR`,
   `classDef` lines for done/inprog/untouched/shadow, SPEC subgraphs, the
   shadow thread nodes, a **"No spec shadow"** section for threads touching
   no contract, and the **"Reading the shape"** narrative paragraph(s) —
   regenerate the narrative from the current shape, don't copy it stale.
6. **Refresh the `Generated <date>` line** to today.
7. **Ship via branch + PR** with the queue row updated in the same PR.

## Boundaries

- **This skill WRITES the tree** — `docs/spec-map.md` only, and only via the
  normal branch + gate + PR flow. Never commit on main; never touch SPECs,
  the registry, or threads themselves (drift you notice goes to `/replan` or
  a report, not fixed here).
- The registry and OPEN-THREADS are the ground truth — the map never
  invents an FR, a status, or a thread; a missing FR shows as a yellow/red
  node, not as a guessed-green one.
- Manual touch-ups between regenerations are allowed (the map header says
  so) — a regeneration supersedes them; flag any it overwrites.
