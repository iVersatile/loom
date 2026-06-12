package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShellUsesExecPathWithTTY pins FR-SHELL-001's mechanism half: shell is
// exec's ONE engine path entered with tty=true and a login shell — same
// lifecycle (start-if-stopped), same workspace cwd, no second code path. The
// interactive EXPERIENCE is human feedback (T1 doctrine); the contract the
// fake records is the automatable proxy.
func TestShellUsesExecPathWithTTY(t *testing.T) {
	root := tempProject(t)
	rec := &execCall{}
	rt := fakeRuntime{exists: true, execRecord: rec}
	res, err := shellImpl(ShellOpts{PlaybookPath: filepath.Join(root, "loom.yml")}, rt, fixedClock)
	if err != nil {
		t.Fatalf("shell: %v", err)
	}
	if !rec.tty {
		t.Error("shell must request a TTY from the runtime")
	}
	if !rec.started {
		t.Error("shell must start a (possibly stopped) container before entering")
	}
	if got := strings.Join(rec.argv, " "); got != "bash -l" {
		t.Errorf("argv = %q, want a login shell (bash -l)", got)
	}
	if rec.workdir != "/workspace/loom" {
		t.Errorf("workdir = %q, want /workspace/loom (project mount)", rec.workdir)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit = %d, want the shell's own code propagated", res.ExitCode)
	}
}

// TestShellAuditsSessionOpenNoCommands pins FR-SHELL-001's audit half: exactly
// one session-open entry, appended at open (Result "opened"), carrying NO
// command — what is typed inside the shell belongs to the session, not the
// log (SPEC-verbs shell).
func TestShellAuditsSessionOpenNoCommands(t *testing.T) {
	root := tempProject(t)
	res, err := shellImpl(ShellOpts{PlaybookPath: filepath.Join(root, "loom.yml")},
		fakeRuntime{exists: true, execExit: 3}, fixedClock)
	if err != nil {
		t.Fatalf("shell: %v", err)
	}
	if res.Action == "" {
		t.Error("ExecResult.Action should carry the session-open audit id")
	}
	if res.ExitCode != 3 {
		t.Errorf("exit = %d, want 3 (the shell's own code)", res.ExitCode)
	}
	data, err := os.ReadFile(filepath.Join(root, ".loom", "actions.log"))
	if err != nil {
		t.Fatalf("read action log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("audit lines = %d, want exactly one session-open entry", len(lines))
	}
	line := lines[0]
	for _, want := range []string{`"verb":"shell"`, `"action":"container.shell"`, `"result":"opened"`} {
		if !strings.Contains(line, want) {
			t.Errorf("audit entry missing %s:\n%s", want, line)
		}
	}
	for _, banned := range []string{"command", "bash"} {
		if strings.Contains(line, banned) {
			t.Errorf("session-open entry must capture no command, found %q:\n%s", banned, line)
		}
	}
}

// TestShellAbsentContainerErrors: the shared lifecycle ruling comes with the
// shared path — absent container errors with the `loom build` hint, shell
// never creates.
func TestShellAbsentContainerErrors(t *testing.T) {
	root := tempProject(t)
	_, err := shellImpl(ShellOpts{PlaybookPath: filepath.Join(root, "loom.yml")},
		fakeRuntime{exists: false}, fixedClock)
	if err == nil || !strings.Contains(err.Error(), "loom build") {
		t.Errorf("absent container should error with the `loom build` hint, got: %v", err)
	}
}
