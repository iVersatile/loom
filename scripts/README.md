# scripts/ — activity scripts (the verb incubator)

Tracked, reviewed shell scripts for operational activities that today happen as
manual command sequences. Committing them does three things:

1. **Records the activity** — the script *is* the durable history of what was
   done and in what order (git history + the script's own `set -x`/echo trail),
   instead of a terminal scrollback that evaporates.
2. **Makes it re-runnable** — idempotent where possible, explicit confirmation
   before destructive steps, never a silent mutation.
3. **Surfaces the gap** — every script here is a *candidate verb or check*. If
   an activity is worth scripting, it is worth asking whether the engine should
   own it (see lifecycle below, and OPEN-THREADS **T17**).

## Conventions

Every script carries a header block:

```
# <name> — one-line purpose
# Origin : the thread/PR/incident that created the need
# Reuse  : one-off | recurring | verb-candidate (promote target if known)
# Runs on: which of the three environments (Mac host / devenv / loom-dev)
```

- POSIX sh, `set -eu`; destructive steps gated on a typed confirmation or an
  explicit `--yes`.
- No secrets in scripts or output (RULES). No `sudo`, no `--no-verify`.
- Scripts may *read* engine artifacts (`loom.lock`, `.loom/`) but must not
  hand-edit them — mutations go through `loom` verbs where one exists.

## Lifecycle (one-off → recurring → engine-owned)

A script's `Reuse` line tracks its trajectory:

- **one-off** — ran once, kept as the record (e.g. a migration).
- **recurring** — reached for a second time; now it must stay current.
- **verb-candidate** — the activity belongs in the engine: a `loom` verb, a
  `doctor` check, or a teardown tier. Promotion follows the loom way: spec
  clause (human-authored, RULES §5/C3) → ADR if a decision is involved → engine
  implementation → FR + test. The script is then deleted with a pointer, not
  left to drift alongside the verb.

The test for promotion: *would a loom **user** (not just a loom developer) need
this?* A migration after a rename — probably yes next time the engine changes
container identity. A leaked-credential sweep — definitely yes (and `detect`'s
credential scan + `teardown --clean-state` are the specced homes for it).

## Index

| script | reuse | promote target |
| --- | --- | --- |
| `migrate-loom-dev.sh` | one-off (this rename) / pattern recurs on any container-identity change | engine: identity migration on rename (T17) |
| `verify-loom-dev.sh` | recurring — every loom-dev session start / post-rebuild | `loom doctor` checks + FRs; GAP list dies with T16; gate-dep claims promote to the Makefile↔playbook joint check (T19) |
