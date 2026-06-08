package engine

import "errors"

// ErrNotImplemented marks a verb whose logic is not yet built. Phase 1 wires the
// full contract surface (flags, exit codes, --json shapes) first, then fills in
// the verbs (RULES §2: contracts before code). The CLI renders the zero-value
// result and notes the stub on stderr rather than faking success.
var ErrNotImplemented = errors.New("not yet implemented (Phase 1 stub)")

// Option structs are the stable inputs each verb takes. They exist now so verb
// signatures don't churn as the logic lands.

type DetectOpts struct {
	EmitPlaybook bool // --emit-playbook (Phase 2 continuity)
	Migrate      bool // --migrate (Phase 2 continuity)
}

type PlanOpts struct{}

type BuildOpts struct {
	Force bool // --force: rebuild from scratch
}

type TeardownOpts struct {
	Level       string // stop | volumes | reset
	CleanState  bool   // --clean-state (Mac-side)
	WipeProject bool   // --wipe-project (typed confirmation)
}

type DoctorOpts struct{}
