package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateIsHermetic proves the RULES §5 hermetic-gate invariant (FR-INV-004) by
// mechanism: the Makefile's unit-test target must scrub ambient override/config env
// (LOOM_*/ALLOW_*) and neutralize docker, so the local gate cannot diverge from CI
// on host env/tooling (LL-006/008). This is the automatable proxy for "the gate is
// hermetic" — testing the mechanism that guarantees it rather than every host.
func TestGateIsHermetic(t *testing.T) {
	mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(mk)
	for _, want := range []string{
		"env -u LOOM_BASE_IMAGE", // ambient base-image override scrubbed
		"-u ALLOW_SPEC_CHANGE",   // audited overrides scrubbed
		"-u ALLOW_TRUST_CHANGE",
		"-u ALLOW_MAIN_COMMIT",
		`"$$d/docker"`, // a failing docker shim is placed on PATH
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the unit gate must scrub env + neutralize docker (FR-INV-004); Makefile missing %q", want)
		}
	}
}
