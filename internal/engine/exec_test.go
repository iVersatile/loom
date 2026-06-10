package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecPropagatesExitCode pins FR-EXEC-001's engine half: the command's
// exit code comes back verbatim — including non-zero — with no error (a
// failing command is a successful exec).
func TestExecPropagatesExitCode(t *testing.T) {
	root := tempProject(t)
	for _, want := range []int{0, 3, 137} {
		rt := fakeRuntime{exists: true, execExit: want}
		res, err := execImpl(ExecOpts{PlaybookPath: filepath.Join(root, "loom.yml"), Command: []string{"true"}}, rt, fixedClock)
		if err != nil {
			t.Fatalf("exec (exit %d): %v", want, err)
		}
		if res.ExitCode != want {
			t.Errorf("exit = %d, want %d (verbatim propagation)", res.ExitCode, want)
		}
	}
}

// TestExecRequiresCommand pins the engine backstop of FR-EXEC-001: an empty
// command is an error, never an interactive fallback.
func TestExecRequiresCommand(t *testing.T) {
	rt := fakeRuntime{exists: true}
	if _, err := execImpl(ExecOpts{Command: nil}, rt, fixedClock); err == nil {
		t.Error("exec without a command must error (usage), not fall back to interactive")
	}
}

// TestExecAbsentContainerErrors pins the lifecycle ruling: absent container →
// error carrying the `loom build` hint; exec never creates.
func TestExecAbsentContainerErrors(t *testing.T) {
	root := tempProject(t)
	rt := fakeRuntime{exists: false}
	_, err := execImpl(ExecOpts{PlaybookPath: filepath.Join(root, "loom.yml"), Command: []string{"true"}}, rt, fixedClock)
	if err == nil || !strings.Contains(err.Error(), "loom build") {
		t.Errorf("absent container should error with the `loom build` hint, got: %v", err)
	}
}

// TestExecTargetsWorkspaceAndStarts pins FR-EXEC-002's engine half plus
// start-if-stopped: the runtime is asked to start (idempotent) and to run the
// argv in /workspace/<project>.
func TestExecTargetsWorkspaceAndStarts(t *testing.T) {
	root := tempProject(t)
	rec := &execCall{}
	rt := fakeRuntime{exists: true, execRecord: rec}
	argv := []string{"go", "version"}
	if _, err := execImpl(ExecOpts{PlaybookPath: filepath.Join(root, "loom.yml"), Command: argv}, rt, fixedClock); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if !rec.started {
		t.Error("exec must start a (possibly stopped) container before entering")
	}
	if rec.workdir != "/workspace/loom" {
		t.Errorf("workdir = %q, want /workspace/loom (project mount)", rec.workdir)
	}
	if strings.Join(rec.argv, " ") != "go version" {
		t.Errorf("argv = %v, want it passed through untouched", rec.argv)
	}
	if rec.name != "loom-dev" {
		t.Errorf("container = %q, want loom-dev", rec.name)
	}
}

// TestExecAuditsCommandAndExit pins FR-EXEC-003: every exec appends an audit
// entry carrying the command and its exit code, and ExecResult carries the
// entry id — the verb's structured surface (no --json, SPEC-verbs exemption).
func TestExecAuditsCommandAndExit(t *testing.T) {
	root := tempProject(t)
	rt := fakeRuntime{exists: true, execExit: 7}
	res, err := execImpl(ExecOpts{PlaybookPath: filepath.Join(root, "loom.yml"), Command: []string{"sh", "-c", "exit 7"}}, rt, fixedClock)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if res.Action == "" {
		t.Error("ExecResult.Action should carry the audit entry id")
	}
	data, err := os.ReadFile(filepath.Join(root, ".loom", "actions.log"))
	if err != nil {
		t.Fatalf("read action log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var e struct {
		Verb   string `json:"verb"`
		Action string `json:"action"`
		After  struct {
			Command string `json:"command"`
			Exit    int    `json:"exit"`
		} `json:"after"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &e); err != nil {
		t.Fatalf("parse last audit line: %v", err)
	}
	if e.Verb != "exec" || e.Action != "container.exec" {
		t.Errorf("audit verb/action = %s/%s, want exec/container.exec", e.Verb, e.Action)
	}
	if e.After.Command != "sh -c exit 7" || e.After.Exit != 7 {
		t.Errorf("audit after = %+v, want the command string and exit 7", e.After)
	}
}
