package engine

// Detect reads current reality and never mutates (docs/SPEC-verbs.md "detect").
// Stub: returns an empty state document; real scanning lands in Work 3.
func Detect(opts DetectOpts) (DetectResult, error) {
	return DetectResult{
		Tools:       []Tool{},
		Agents:      []Agent{},
		Credentials: []Credential{},
		Projects:    []Project{},
		Drift:       []Drift{},
	}, ErrNotImplemented
}
