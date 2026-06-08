package engine

// Teardown removes the environment in tiers: stop | volumes | reset
// (docs/SPEC-verbs.md "teardown"). Stub: real removal lands in Work 6.
func Teardown(opts TeardownOpts) (TeardownResult, error) {
	return TeardownResult{
		Level:   opts.Level,
		Removed: Removed{Containers: []string{}, Volumes: []string{}, Images: []string{}},
	}, ErrNotImplemented
}
