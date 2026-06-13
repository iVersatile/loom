# RULES — How we work on Loom

Source of truth for contributors (human and AI). Numbered for citation.

## §1 Branching
- Never commit on main/master (branch-guard enforces). Use `feat/<slug>`,
  `fix/<slug>`, `docs/<slug>`. Override only with `ALLOW_MAIN_COMMIT=1` (audited).

## §2 Specs before code (ADR-0006)
- Schema and verb contracts change via SPEC edits + review BEFORE implementation.
- Code that contradicts a frozen spec is a bug in the code, not the spec.

## §3 Decisions
- Architectural choices get an ADR (context/decision/alternatives/consequences).
- Supersede ADRs; never silently reverse one.

## §4 Lessons
- Record bugs/regressions in docs/LESSONS_LEARNT.md (LL-NNN, append-only, tagged).
- Before fixing a bug, grep lessons for the surface-area tag; cite `Applying LL-NNN`.

## §5 AI-first invariants (ADR-0005) — non-negotiable
- Every verb: human + `--json` output.
- Every mutating verb: idempotent, recoverable, audit-logged.
- Guardrails enforced by mechanism, never trust. Agents cannot weaken them.
- Design test: "could an agent do this unattended, and would guardrails hold if
  it tried the worst thing?"
- The unit gate is hermetic: same result regardless of host tooling/env
  (no dependence on ambient PATH binaries, docker presence, or
  LOOM_*/ALLOW_*). The agent's local gate equals CI.

## §6 Hard constraints
- No hardcoded credentials. No `--dangerously-skip-permissions`, `--no-verify`,
  `sudo`. No secrets in stdout/stderr or committed files. No force-push to main.

## §7 Gates
- Lint/type/test/secret-scan run as local pre-commit; adversarial review + heavy
  tests + security run in CI (advisory review, non-blocking). Don't bypass.

## §8 Dogfooding
- Loom is built using Loom from the first runnable slice. If a gap hurts us, it
  would hurt an autonomous agent more — fix it.

## §9 Design methodology — Close → Smooth → Skillify
- Design any subsystem in three ordered phases; each produces the next's input:
  1. **Close the circle** — make the loop complete: every state has a next state,
     nothing dropped, no dead-ends. Correctness before performance — slow,
     human-gated, or poll-based is fine here. (Eliminates gaps.)
  2. **Smooth it** — run it repeatedly. Re-use validates purpose (a unit reused
     across contexts is a real abstraction, not a one-off); repetition exposes
     performance (bottlenecks a single run hides). (Validates units + surfaces cost.)
  3. **Skillify** — turn each validated step into a named, contracted skill:
     enhanceable against a clear benchmark in isolation, auditable on failure.
- Order is load-bearing — smoothing reveals which boundaries are real and the
  benchmarks to skillify against. Don't reorder: smooth-before-close polishes a
  leaky loop; skillify-before-smooth freezes unvalidated joints; skillify-before-
  close ships transparency over a broken process.
- A subsystem isn't "done" until its hot path is skills with benchmarks — phase 3
  is the one teams skip, accumulating smooth-but-opaque monoliths.
