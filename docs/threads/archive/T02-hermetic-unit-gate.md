# T2 — Hermetic unit gate   ✅ resolved

Archived from docs/OPEN-THREADS.md on resolution (T21-era convention:
archive on resolution; the stub in OPEN-THREADS carries the pointers).

Origin: LL-006 / LL-008 — three CI reds this session were unit tests that passed
locally and failed in CI because the local box lacked an env var / a binary CI had.

**Decisions.**
- **Q2.1 (A)** the local `make gate` scrubs the env by default, so **local ≡ CI**.
- **Q2.2 (A)** detection = run the unit suite in a *targeted-scrubbed* env (unset
  `LOOM_*` / `ALLOW_*`, make docker unavailable; keep the gate's own toolchain on
  `PATH`). NOT a static lint.
- **Q2.3 — PARKED** (was "yes"). Making "the unit gate is hermetic" an invariant
  FR needs a spec clause to cite (ADR-0013 `spec → FR`), but C3 forbids the AI
  authoring that RULES §5 clause. So: a **human** authors the RULES §5 hermetic
  invariant if/when they want the FR; until then no `FR-INV` for it. The mechanism
  ships regardless (below).

**Mechanism ships as plain hardening (not an FR):** Q2.1 + Q2.2 collapse into *one*
change — the gate runs unit tests in a targeted-scrubbed env, everywhere — which
directly kills the LL-006/008 class that hit 3× this session. The thing that *would*
be overkill (the AST lint, Q2.2 option b) is excluded. Impl note: "scrub" is
targeted (unset `LOOM_*`/`ALLOW_*` + hide docker), **not** `env -i` — the gate needs
go/gofmt/golangci-lint/gitleaks on PATH.

**Scope extended (2026-06-10, LL-010):** the targeted scrub also unsets the
`GIT_*` repo-redirection vars (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`,
`GIT_OBJECT_DIRECTORY`, `GIT_COMMON_DIR`) — a leaked `GIT_DIR` overrides both
cwd and `-C` (verified), so git-shelling fixtures wrote into the real shared
`.git` (incident postmortem: LL-010). Fixtures are additionally hermetic on
their own (`hermeticEnv()` + explicit `-C`), pinned by
`TestGateHermeticToGitEnv`.
