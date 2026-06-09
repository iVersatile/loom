# Rule: common/safety

Hard safety constraints for every Loom environment (base tier).

- No hardcoded credentials; env vars or a secret manager only.
- No `--dangerously-skip-permissions`, `--no-verify`, or `sudo`.
- No secrets in stdout/stderr or committed files.
- No force-push to `main`/`master`.

Enforced by mechanism via the `guard-bash`, `branch-guard`, and `protect-paths`
hooks — not by trust (ADR-0005).
