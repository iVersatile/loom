package engine

// Build materializes the playbook into reality: resolve intent → write lockfile
// → produce the container + two-tier config (docs/SPEC-verbs.md "build"). It is
// the first mutating verb, so it owns the audit-log append site. Stub: the real
// pipeline lands in Work 5.
func Build(opts BuildOpts) (BuildResult, error) {
	return BuildResult{
		Resolved:     map[string]ResolvedTool{},
		Materialized: []string{},
		Actions:      []string{},
		Result:       "noop",
	}, ErrNotImplemented
}
