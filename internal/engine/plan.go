package engine

// Plan computes the diff between current state and the playbook; never mutates
// (docs/SPEC-verbs.md "plan"). Stub: returns a converged (empty) plan; the real
// diff lands in Work 3.
func Plan(opts PlanOpts) (PlanResult, error) {
	return PlanResult{
		Create:  []CreateItem{},
		Install: []InstallItem{},
		Remove:  []RemoveItem{},
		Noop:    []string{},
	}, ErrNotImplemented
}
