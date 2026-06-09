package engine

import "testing"

func TestDoctorChecks(t *testing.T) {
	// All tools present, fixture ships executable hooks, but no loom.lock → the
	// lockfile check should fail while tools/hooks pass.
	p := fakeProber{"git": "2.43", "jq": "1.7", "go": "go1.26.4", "gopls": "0.16"}
	res, err := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, p)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}

	byName := map[string]Check{}
	for _, c := range res.Checks {
		byName[c.Name] = c
	}

	if c, ok := byName["tool:go"]; !ok || !c.OK {
		t.Errorf("tool:go should pass, got %+v", c)
	}
	for _, h := range []string{"hook:guard-bash", "hook:branch-guard", "hook:protect-paths"} {
		if c, ok := byName[h]; !ok || !c.OK {
			t.Errorf("%s should pass (present+executable), got %+v", h, c)
		}
	}
	if c, ok := byName["lockfile"]; !ok || c.OK {
		t.Errorf("lockfile check should fail without loom.lock, got %+v", c)
	}
	if res.OK() {
		t.Error("doctor should not be OK with a missing lockfile")
	}
}

func TestDoctorMissingTool(t *testing.T) {
	// gopls absent → its check fails.
	p := fakeProber{"git": "2.43", "jq": "1.7", "go": "go1.26.4"}
	res, _ := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, p)
	for _, c := range res.Checks {
		if c.Name == "tool:gopls" && c.OK {
			t.Error("tool:gopls should fail when absent")
		}
	}
}
