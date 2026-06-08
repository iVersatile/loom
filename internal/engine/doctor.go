package engine

// Doctor self-checks the environment: tools present, hooks executable, lockfile
// consistent, guardrails active (docs/SPEC-verbs.md "doctor / verify"). Stub:
// real checks land in Work 6 (it is the assertion surface for the guardrail test).
func Doctor(opts DoctorOpts) (DoctorResult, error) {
	return DoctorResult{Checks: []Check{}}, ErrNotImplemented
}
