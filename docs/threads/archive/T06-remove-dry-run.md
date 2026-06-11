# T6 — `build --dry-run` mutates (violates plan-semantics contract)   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

Origin: a Mac `./bin/loom build --dry-run` that was expected to preview but actually
provisioned.

`--dry-run` is documented as "preview changes without applying (plan semantics)"
(top-level flag help) and `plan` is the never-mutates verb (`SPEC-verbs.md`). But a
`build --dry-run` run did real work:
- reported `created (container loom-loom-dev, lock_written=true, 3 materialized)`;
- actually provisioned — `.loom/logs/build.log` shows real `go: downloading …` and
  `+ grep -q bashrc.d /root/.bashrc` (container commands executed);
- rewrote `loom.lock` (`resolved_at` advanced) and wrote staging files under
  `.loom/home/…`.

A dry-run must do none of these. Either `build` ignores the `--dry-run` flag, or it
threads it but the container/materialize/lock-write path doesn't honor it. Net
effects: false "preview" that mutates state, and (combined with T5) it wrote a bad
lock unprompted.

Next step: trace where `--dry-run` is read in the `build` path (cmd/loom + the build
engine) and confirm whether the flag reaches the mutating steps at all. Likely a
guard missing before container create / materialize / lock-write.

**Correction to an earlier audit note:** `--dry-run` WAS specced — SPEC-verbs
*Global conventions* ("`--dry-run` where an action would change state (alias of
`plan` semantics)") and `update` ("`--dry-run` == `plan`"). The thread title was
right all along: a contract violation, not unspecced growth. (The earlier "spec
the flag" direction was based on the wrong unspecced reading and is superseded.)

**Root cause (confirmed in PR #8):** the flag was registered as a persistent flag
in `internal/cli/root.go` but **no verb ever read it** — `build` ran its full
mutating path unconditionally. A promise with no mechanism.

**Resolution (user decision, 2026-06-09): abandon `--dry-run`; `plan` is the one
preview path.** Implemented in **PR #8** (`fix/t6-remove-dry-run`): the flag is
removed from the CLI, and SPEC-verbs is amended (audited `ALLOW_SPEC_CHANGE` on
explicit instruction; merge = human acceptance) — the global convention now states
"No `--dry-run`" with the T6 rationale, and `update` points at `plan`. One preview
surface keeps the read-only promise enforceable; it stays covered by FR-PLAN-001/002
(no FR cited the removed clause; `fr-verify` green).
