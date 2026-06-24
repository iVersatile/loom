package engine

import (
	"path/filepath"
	"testing"
)

// TestMutatingVerbsIdempotent is the parametrized proof of the RULES §5 invariant
// "every mutating verb is idempotent: re-running converges" (FR-INV-003). For each
// mutating verb, the first run mutates and audits; a second run against the now-
// converged state appends NO new audit entries. Mechanism, not trust: break a
// verb's idempotency and this fails for that verb.
func TestMutatingVerbsIdempotent(t *testing.T) {
	cases := []struct {
		verb string
		// run a verb against root; first=true is the initial mutation, first=false
		// the re-run against the now-converged state.
		run func(t *testing.T, root string, first bool)
	}{
		{"build", func(t *testing.T, root string, first bool) {
			status := "exists"
			if first {
				status = "created"
			}
			rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: status}}
			if _, err := buildImpl(BuildOpts{PlaybookPath: filepath.Join(root, "loom.yml")}, rt, fixedClock); err != nil {
				t.Fatalf("build: %v", err)
			}
		}},
		{"teardown", func(t *testing.T, root string, first bool) {
			removed := Removed{}
			if first {
				removed = Removed{Containers: []string{"loom-dev"}}
			}
			rt := fakeRuntime{teardownRemoved: removed}
			if _, err := teardownImpl(TeardownOpts{PlaybookPath: filepath.Join(root, "loom.yml"), Level: "stop"}, rt, fixedClock, no); err != nil {
				t.Fatalf("teardown: %v", err)
			}
		}},
		{"update", func(t *testing.T, root string, first bool) {
			// First run: no container yet → plan reports a delta → update applies it
			// (lock + home + container audits). Re-run: the now-converged container
			// (built host side, every tool/agent present, home synced) → plan is a
			// noop → update writes nothing (the reachable "noop", R6).
			var rt fakeRuntime
			if first {
				rt = fakeRuntime{exists: false, ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}, probeVersions: fixtureProbes()}
			} else {
				rt = fakeRuntime{exists: true, running: true, probeVersions: fixtureProbes(),
					homeSentinel: homeDigest(filepath.Join(root, ".loom", "home"))}
			}
			if _, err := updateImpl(UpdateOpts{PlaybookPath: filepath.Join(root, "loom.yml")}, rt, fixedClock); err != nil {
				t.Fatalf("update: %v", err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.verb, func(t *testing.T) {
			root := tempProject(t)
			c.run(t, root, true)
			before := countLogLines(t, root)
			c.run(t, root, false)
			if after := countLogLines(t, root); after != before {
				t.Errorf("%s not idempotent: re-run appended %d audit entries, want 0", c.verb, after-before)
			}
		})
	}
}
