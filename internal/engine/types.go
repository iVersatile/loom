// Package engine is the mechanism (ADR-0006): it reconciles reality to the
// playbook. This file defines the typed results every verb returns; their JSON
// tags ARE the --json contract (docs/SPEC-verbs.md). Verb logic lands in the
// per-verb files (detect.go, plan.go, ...); Phase 1 starts with stubs.
package engine

import (
	"fmt"
	"strings"
)

// --- detect ---------------------------------------------------------------

type Tool struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Version string `json:"version,omitempty"`
}

type Agent struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
}

type Credential struct {
	Name    string   `json:"name"`
	FoundIn []string `json:"found_in"`
}

type Project struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Stack string `json:"stack"`
}

// Drift is one tool whose installed state differs from the playbook. Have is a
// pointer so "not installed" serializes as JSON null, distinct from "".
type Drift struct {
	Tool string  `json:"tool"`
	Want string  `json:"want"`
	Have *string `json:"have"`
}

type DetectResult struct {
	Tools       []Tool       `json:"tools"`
	Agents      []Agent      `json:"agents"`
	Credentials []Credential `json:"credentials"`
	Projects    []Project    `json:"projects"`
	Drift       []Drift      `json:"drift"`
}

func (r DetectResult) Human() string {
	var b strings.Builder
	fmt.Fprintf(&b, "detect: %d tools, %d agents, %d credentials, %d projects, %d drift",
		len(r.Tools), len(r.Agents), len(r.Credentials), len(r.Projects), len(r.Drift))
	return b.String()
}

// --- plan -----------------------------------------------------------------

type CreateItem struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// InstallItem is a tool to install or change. From is a pointer so a fresh
// install serializes as JSON null rather than "".
type InstallItem struct {
	Tool string  `json:"tool"`
	From *string `json:"from"`
	To   string  `json:"to"`
}

type RemoveItem struct {
	Tool   string `json:"tool"`
	Reason string `json:"reason"`
}

type PlanResult struct {
	Create  []CreateItem  `json:"create"`
	Install []InstallItem `json:"install"`
	Remove  []RemoveItem  `json:"remove"`
	Noop    []string      `json:"noop"`
	// Target is the graded container, for the human line (guided-run finding
	// ⑤: a verdict that doesn't name its target can't be safety-checked
	// without --json). json:"-": the spec'd --json shape is frozen.
	Target string `json:"-"`
}

// Changed reports whether applying the plan would mutate state. It drives the
// plan exit code: 2 when true, 0 when converged (docs/SPEC-verbs.md).
func (r PlanResult) Changed() bool {
	return len(r.Create)+len(r.Install)+len(r.Remove) > 0
}

func (r PlanResult) Human() string {
	target := ""
	if r.Target != "" {
		target = " " + r.Target
	}
	return fmt.Sprintf("plan%s: +%d create, %d install, -%d remove, %d noop",
		target, len(r.Create), len(r.Install), len(r.Remove), len(r.Noop))
}

// --- build ----------------------------------------------------------------

type ResolvedTool struct {
	Resolved string `json:"resolved"`
	Source   string `json:"source"`
}

type ContainerInfo struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
}

type BuildResult struct {
	Resolved     map[string]ResolvedTool `json:"resolved"`
	LockWritten  bool                    `json:"lock_written"`
	Container    ContainerInfo           `json:"container"`
	Materialized []string                `json:"materialized"`
	Actions      []string                `json:"actions"`
	Result       string                  `json:"result"` // created | converged | noop

	// LogPath is the diagnostic log for this run. Excluded from --json (the
	// shape is frozen); the CLI surfaces it on stderr for troubleshooting.
	LogPath string `json:"-"`
}

func (r BuildResult) Human() string {
	return fmt.Sprintf("build: %s (container %s, lock_written=%t, %d materialized)",
		r.Result, r.Container.Name, r.LockWritten, len(r.Materialized))
}

// --- teardown -------------------------------------------------------------

type Removed struct {
	Containers []string `json:"containers"`
	Volumes    []string `json:"volumes"`
	Images     []string `json:"images"`
}

type TeardownResult struct {
	Level   string  `json:"level"`
	Removed Removed `json:"removed"`

	// LogPath is the diagnostic log for this run (json:"-", no shape change).
	LogPath string `json:"-"`
}

func (r TeardownResult) Human() string {
	// Level is quoted so the line reads as a report, not a "teardown stop:"
	// prefix label (guided-run finding ⑨ ergonomics note).
	return fmt.Sprintf("teardown level %q: removed %d containers, %d volumes, %d images",
		r.Level, len(r.Removed.Containers), len(r.Removed.Volumes), len(r.Removed.Images))
}

// --- doctor ---------------------------------------------------------------

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type DoctorResult struct {
	Checks []Check `json:"checks"`
}

// OK reports whether every check passed.
func (r DoctorResult) OK() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

func (r DoctorResult) Human() string {
	var b strings.Builder
	pass := 0
	for _, c := range r.Checks {
		mark := "ok"
		if !c.OK {
			mark = "FAIL"
		} else {
			pass++
		}
		fmt.Fprintf(&b, "[%s] %s", mark, c.Name)
		if c.Detail != "" {
			fmt.Fprintf(&b, " — %s", c.Detail)
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "doctor: %d/%d checks passed", pass, len(r.Checks))
	return b.String()
}
