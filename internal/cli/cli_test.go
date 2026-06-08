package cli

import (
	"bytes"
	"encoding/json"
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
	out, errs, err := runCmd(t, "detect", "--json")
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
	// Stub honesty: the note belongs on stderr, never polluting --json stdout.
	if !strings.Contains(errs, "stub") {
		t.Errorf("expected a stub note on stderr, got %q", errs)
	}
}

func TestConvergedPlanNoError(t *testing.T) {
	// The stub plan is converged → no error → Execute would map to exit 0.
	if _, _, err := runCmd(t, "plan", "--json"); err != nil {
		t.Fatalf("converged plan should not error, got %v", err)
	}
}

func TestTeardownInvalidLevel(t *testing.T) {
	_, _, err := runCmd(t, "teardown", "bogus")
	if err == nil {
		t.Fatal("expected an error for an invalid teardown level")
	}
}

func TestTeardownValidLevels(t *testing.T) {
	for _, lvl := range []string{"stop", "volumes", "reset"} {
		if _, _, err := runCmd(t, "teardown", lvl, "--json"); err != nil {
			t.Errorf("teardown %s: unexpected error %v", lvl, err)
		}
	}
}

func TestUnknownVerbErrors(t *testing.T) {
	if _, _, err := runCmd(t, "frobnicate"); err == nil {
		t.Fatal("unknown verb should return an error (exit 1)")
	}
}
