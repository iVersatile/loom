package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// runCmd executes the real command tree with captured stdout/stderr.
func runCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestDetectJSONContract(t *testing.T) {
	out, _, err := runCmd(t, "detect", "--json", "-f", fixturePlaybook)
	if err != nil {
		t.Fatalf("detect: unexpected error %v", err)
	}
	var got map[string]any
	if e := json.Unmarshal([]byte(out), &got); e != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", e, out)
	}
	for _, k := range []string{"tools", "agents", "credentials", "projects", "drift"} {
		if _, ok := got[k]; !ok {
			t.Errorf("detect --json missing key %q", k)
		}
	}
}

func TestPlanEmitsJSONAndExitCode(t *testing.T) {
	// Against the fixture (nothing built), plan reports drift → valid JSON on
	// stdout and an exit-2 signal, never a hard error.
	out, _, err := runCmd(t, "plan", "--json", "-f", fixturePlaybook)
	var ee *exitError
	if err != nil && !errors.As(err, &ee) {
		t.Fatalf("plan should only fail via exit code, got %v", err)
	}
	if ee != nil && ee.code != 2 {
		t.Errorf("plan drift exit code = %d, want 2", ee.code)
	}
	if !json.Valid([]byte(out)) {
		t.Errorf("plan --json stdout is not valid JSON:\n%s", out)
	}
}

func TestPlanMissingPlaybookErrors(t *testing.T) {
	// No playbook at the given path → a real error (exit 1), not exit 2.
	_, _, err := runCmd(t, "plan", "--json", "-f", "testdata/does-not-exist.yml")
	if err == nil {
		t.Fatal("plan with no playbook should error")
	}
	var ee *exitError
	if errors.As(err, &ee) {
		t.Errorf("missing playbook should be a hard error, not an exit-code signal")
	}
}

func TestTeardownInvalidLevel(t *testing.T) {
	_, _, err := runCmd(t, "teardown", "bogus")
	if err == nil {
		t.Fatal("expected an error for an invalid teardown level")
	}
}

func TestTeardownValidLevels(t *testing.T) {
	proj := tempCopy(t, "../playbook/testdata/proj") + "/loom.yml"
	for _, lvl := range []string{"stop", "volumes", "reset"} {
		// docker is absent here, so a container-step error is expected; only an
		// "invalid level" rejection would be a real bug. --yes: tests run
		// without a TTY, and unattended teardown states intent explicitly.
		_, _, err := runCmd(t, "teardown", lvl, "-f", proj, "--json", "--yes")
		if err != nil && strings.Contains(err.Error(), "invalid level") {
			t.Errorf("teardown %s wrongly rejected: %v", lvl, err)
		}
	}
}

// Consent gate (FR-TEARDOWN-003, guided-run finding ⑨): the most destructive
// routine verb never acts on silence — operators answer the prompt, automation
// says --yes, and an unanswerable prompt (EOF) removes nothing.
func TestTeardownRequiresConsent(t *testing.T) {
	proj := tempCopy(t, "../playbook/testdata/proj") + "/loom.yml"
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetIn(strings.NewReader("")) // unattended: nobody to answer
	root.SetArgs([]string{"teardown", "stop", "-f", proj})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("unanswered teardown without --yes should refuse naming the remedy, got %v", err)
	}
}

func TestTeardownPromptDecides(t *testing.T) {
	proj := tempCopy(t, "../playbook/testdata/proj") + "/loom.yml"

	for _, tc := range []struct {
		answer  string
		aborted bool
	}{{"n\n", true}, {"\n", true}, {"y\n", false}, {"YES\n", false}} {
		root := newRootCmd()
		var out, errb bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errb)
		root.SetIn(strings.NewReader(tc.answer))
		root.SetArgs([]string{"teardown", "stop", "-f", proj, "--json"})
		err := root.Execute()
		gotAborted := err != nil && strings.Contains(err.Error(), "aborted")
		if gotAborted != tc.aborted {
			t.Errorf("answer %q: aborted = %v, want %v (err: %v)", tc.answer, gotAborted, tc.aborted, err)
		}
		if !strings.Contains(errb.String(), "[y/N]") {
			t.Errorf("answer %q: expected a [y/N] prompt on stderr", tc.answer)
		}
	}
}

func TestUnknownVerbErrors(t *testing.T) {
	if _, _, err := runCmd(t, "frobnicate"); err == nil {
		t.Fatal("unknown verb should return an error (exit 1)")
	}
}

// TestShellRejectsArgs pins FR-SHELL-001's CLI half: shell takes no command —
// exec is the command door; `loom shell <cmd>` is a usage error.
func TestShellRejectsArgs(t *testing.T) {
	if _, _, err := runCmd(t, "shell", "ls"); err == nil {
		t.Fatal("shell with arguments should be a usage error")
	}
}

// TestExecRequiresCommand pins FR-EXEC-001's CLI half (SPEC-verbs exec): a
// bare `loom exec` (or `loom exec --`) is a usage error — non-zero exit,
// never an interactive fallback, no engine call needed to fail.
func TestExecRequiresCommand(t *testing.T) {
	for _, args := range [][]string{{"exec"}, {"exec", "--"}} {
		_, _, err := runCmd(t, args...)
		if err == nil {
			t.Errorf("loom %s should be a usage error, not an interactive fallback", strings.Join(args, " "))
		}
		if err != nil && !strings.Contains(err.Error(), "exec requires a command") {
			t.Errorf("loom %s error = %q, want the usage message", strings.Join(args, " "), err)
		}
	}
}
