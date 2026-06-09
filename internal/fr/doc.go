// Package fr holds the FR-registry traceability check (ADR-0013). It carries no
// runtime code — the check lives in verify_test.go behind the `frcheck` build tag,
// run via `make fr-verify` and the CI boundary job, NOT the per-commit gate
// (advisory by default, blocking at the boundary).
package fr
