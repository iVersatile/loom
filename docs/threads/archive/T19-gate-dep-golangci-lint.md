# T19 — gate dependency golangci-lint undeclared and undeclarable   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

**Resolution (2026-06-10):** everything operational shipped and verified —
tool fix merged (#19: sourcePolicy + goModule + base playbook + tests),
mechanism (a) merged (#17: claims script probes gate deps the
Makefile-resolution way), lock container-re-pinned (#22), recurrence handled
(stale-binary note above; binaries rebuilt), closing quiz 7/7 with the gate
toolchain engine-guaranteed. What does NOT keep this open: mechanism (b), the
Makefile↔playbook joint check, stays a recorded design in the promote-to by
explicit choice — it graduates via its own queue row when prioritized, not by
holding this thread open. Entry kept for the analysis and (b)'s design.
Origin: **human exploration + out-of-band advisory analysis from the old
environment** (a missing brew formula prompted the trace) — pointedly NOT the
claims script and NOT the gate. That attribution matters: both mechanisms that
exist to catch environment drift were blind to this one, which is the actual
finding.

**The defect (verified in-container, 2026-06-10).** `make gate` hard-fails
without golangci-lint (Makefile resolves it via `command -v` and `lint` exits 1
if absent), yet:
- no playbook tier declared it; it is absent from `loom.lock` and from the
  freshly built `loom-dev`;
- it COULDN'T be naively declared: `goModule()`
  (internal/engine/container.go) mapped only gopls and gitleaks, so a bare
  `golangci-lint` entry would fall to the resolver's default **apt** source
  (internal/resolver/resolver.go `sourceFor`), and debian bookworm ships no
  golangci-lint package — the provision would have broken.
- **Masking:** in `devenv` it existed by accident (`~/go/bin`, hand-installed
  history), so the omission was invisible for the whole pre-cutover period.

**Fix shipped (branch `fix/gate-dep-golangci-lint`).** sourcePolicy gains
`golangci-lint: go-install`; `goModule()` maps it to
`github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`; the base
playbook declares it in `tools:`; resolver + provision-script tests extended
(mirroring the gitleaks mappings). The provision digest changes, so the next
build re-provisions existing containers — expected and non-destructive
(converge, not `--force`; creds volume unaffected); `loom.lock` re-pins on
that host-side build.

**The two mechanism gaps (why nothing caught it).**
- **(a) The claims script doesn't check gate dependencies.**
  `scripts/verify-loom-dev.sh` probes the playbook-declared tool list — a tool
  the playbook never declared is structurally invisible to it. Fix (shipped on
  `feat/verify-loom-dev-claims`): probe golangci-lint in the PRESERVE loop and
  assert every Makefile-resolved gate binary exists.
- **(b) Nothing asserts gate-deps ⊆ playbook-declared tools.** The Makefile
  and the playbook can drift silently — the same class of joint the spec↔FR
  check (`fr-verify`, T3/ADR-0013) exists to guard. Today the joint is
  enforced nowhere: not at build, not in the gate, not in CI.

**Promote to (lean — design only, not built).** Mechanism (b) as a
verify-style joint check: parse the Makefile's required binaries (the
`command -v`-resolved set), assert each is playbook-declared (hence locked and
provisioned). Tiering per ADR-0013 C4, exactly like `fr-verify`: **advisory in
`make gate`, blocking at the merge boundary**, never per-commit. Open
questions for that design: where the "gate deps" set is authoritatively
declared (parse the Makefile vs a small manifest the Makefile and check both
read), and whether the joint generalizes to "any repo-declared workflow dep ⊆
playbook tools" (e.g. the pre-commit hook's own needs — `make` itself was the
same defect, caught the same day; see T18).

**Recurrence (2026-06-10, same day, the stale-$LOOM class).** The post-merge
`loom build` on the Mac silently skipped the expected re-pin: `bin/loom*` were
built at 09:17, the T19 fix merged at 13:55 (`grep -ac golangci-lint` = 0 on
both binaries — the resolver change wasn't in them). Second stale-binary hit
today (the first is this thread's origin note; PR #14 guarded the migration
script against exactly this). Binaries rebuilt from `7bf5988`, grep-verified.
Consider generalizing the migration script's `grep -aq` guard (T17): `build`
could self-check binary-vs-tree currency (embedded commit stamp compared to
the working tree's HEAD) and warn, instead of each script reinventing the
grep.
