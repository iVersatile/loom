package engine

import "errors"

// ErrNotImplemented marks a verb whose logic is not yet built. Phase 1 wires the
// full contract surface (flags, exit codes, --json shapes) first, then fills in
// the verbs (RULES §2: contracts before code). The CLI renders the zero-value
// result and notes the stub on stderr rather than faking success.
var ErrNotImplemented = errors.New("not yet implemented (Phase 1 stub)")

// defaultPlaybookPath is the project playbook looked up when --playbook is unset.
const defaultPlaybookPath = "loom.yml"

// Default probe sets used when no playbook narrows them (e.g. detect on a bare
// machine with no project).
var (
	defaultProbeTools = []string{"git", "jq", "ripgrep"}
	defaultAgents     = []string{"claude-code", "codex", "gemini"}
)

// Option structs are the stable inputs each verb takes. They exist now so verb
// signatures don't churn as the logic lands.

type DetectOpts struct {
	PlaybookPath string // --playbook/-f; falls back to defaultPlaybookPath
	EmitPlaybook bool   // --emit-playbook (Phase 2 continuity)
	Migrate      bool   // --migrate (Phase 2 continuity)
}

type PlanOpts struct {
	PlaybookPath string // --playbook/-f; falls back to defaultPlaybookPath
}

type BuildOpts struct {
	PlaybookPath string // --playbook/-f; falls back to defaultPlaybookPath
	Force        bool   // --force: rebuild from scratch
}

type TeardownOpts struct {
	PlaybookPath string // --playbook/-f; falls back to defaultPlaybookPath
	Level        string // stop | volumes | reset
	CleanState   bool   // --clean-state (Mac-side)
	WipeProject  bool   // --wipe-project (typed confirmation)
}

type DoctorOpts struct {
	PlaybookPath string // --playbook/-f; falls back to defaultPlaybookPath
}

type ExecOpts struct {
	PlaybookPath string   // --playbook/-f; falls back to defaultPlaybookPath
	Command      []string // argv to run inside the container; required (SPEC-verbs exec)
}
