package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checksByName indexes a result for assertion convenience.
func checksByName(res DoctorResult) map[string]Check {
	byName := map[string]Check{}
	for _, c := range res.Checks {
		byName[c.Name] = c
	}
	return byName
}

func TestDoctorChecks(t *testing.T) {
	// Running container with all tools probed present, fixture ships executable
	// hooks, but no loom.lock → the lockfile check should fail while
	// container-tier tools and host-tier hooks pass.
	rt := fakeRuntime{exists: true, running: true,
		probeVersions: map[string]string{"git": "2.43", "jq": "1.7", "go": "go1.26.4", "gopls": "0.16"}}
	res, err := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, rt)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}

	byName := checksByName(res)
	if c, ok := byName["container:tool:go"]; !ok || !c.OK {
		t.Errorf("container:tool:go should pass, got %+v", c)
	}
	if c, ok := byName["container:state"]; !ok || !c.OK {
		t.Errorf("container:state should pass for a running container, got %+v", c)
	}
	for _, h := range []string{"host:hook:guard-bash", "host:hook:branch-guard", "host:hook:protect-paths"} {
		if c, ok := byName[h]; !ok || !c.OK {
			t.Errorf("%s should pass (present+executable), got %+v", h, c)
		}
	}
	if c, ok := byName["host:lockfile"]; !ok || c.OK {
		t.Errorf("host:lockfile check should fail without loom.lock, got %+v", c)
	}
	if res.OK() {
		t.Error("doctor should not be OK with a missing lockfile")
	}
}

func TestDoctorMissingTool(t *testing.T) {
	// gopls absent from the container → its check fails.
	rt := fakeRuntime{exists: true, running: true,
		probeVersions: map[string]string{"git": "2.43", "jq": "1.7", "go": "go1.26.4"}}
	res, _ := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, rt)
	c, ok := checksByName(res)["container:tool:gopls"]
	if !ok || c.OK {
		t.Errorf("container:tool:gopls should fail when absent, got %+v", c)
	}
}

// TestDoctorAbsentContainer: with no container there is nothing to grade —
// container:state fails with the remedy and NO per-tool checks appear (finding
// ⑧: doctor must never invent a container verdict from the host PATH).
func TestDoctorAbsentContainer(t *testing.T) {
	res, err := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, fakeRuntime{exists: false})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	c, ok := checksByName(res)["container:state"]
	if !ok || c.OK || !strings.Contains(c.Detail, "absent") {
		t.Errorf("container:state should fail naming the absence, got %+v", c)
	}
	for _, c := range res.Checks {
		if strings.HasPrefix(c.Name, "container:tool:") {
			t.Errorf("no container → no tool verdicts, got %+v", c)
		}
	}
	if res.OK() {
		t.Error("doctor should not be OK without a container")
	}
}

// TestDoctorStoppedContainerGradesFromLock: doctor never mutates, so it must
// not Start a stopped container to probe it — the lock's container-pinned
// `resolved` values (T5) are the fallback truth, and container:state says so.
func TestDoctorStoppedContainerGradesFromLock(t *testing.T) {
	dir := tempProject(t)
	lockBody := `{"loom_lock":1,"resolved_at":"2026-06-11T00:00:00Z","base_image":"debian@sha256:x",
"tools":{"git":{"intent":"latest","resolved":"2.43","source":"apt"},
"jq":{"intent":"latest","resolved":"1.7","source":"apt"},
"go":{"intent":"1.26","resolved":"go1.26.4","source":"image"},
"gopls":{"intent":"latest","resolved":"","source":"go"}},"agents":{}}`
	if err := os.WriteFile(filepath.Join(dir, "loom.lock"), []byte(lockBody), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := doctorImpl(DoctorOpts{PlaybookPath: filepath.Join(dir, "loom.yml")},
		fakeRuntime{exists: true, running: false})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	byName := checksByName(res)
	if c := byName["container:state"]; !c.OK || !strings.Contains(c.Detail, "from lock") {
		t.Errorf("container:state should pass and name the lock as source, got %+v", c)
	}
	if c := byName["container:tool:go"]; !c.OK || c.Detail != "go1.26.4" {
		t.Errorf("container:tool:go should pass with the lock-pinned version, got %+v", c)
	}
	if c := byName["container:tool:gopls"]; c.OK {
		t.Errorf("container:tool:gopls should fail (empty lock pin), got %+v", c)
	}
}

// TestDoctorRuntimeUnavailable: when the runtime cannot even be asked, the
// container tier says so instead of guessing — and emits no tool verdicts.
func TestDoctorRuntimeUnavailable(t *testing.T) {
	res, err := doctorImpl(DoctorOpts{PlaybookPath: testFixture},
		fakeRuntime{err: os.ErrPermission})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	c, ok := checksByName(res)["container:state"]
	if !ok || c.OK || !strings.Contains(c.Detail, "runtime unavailable") {
		t.Errorf("container:state should fail naming the runtime, got %+v", c)
	}
	for _, c := range res.Checks {
		if strings.HasPrefix(c.Name, "container:tool:") {
			t.Errorf("unreachable runtime → no tool verdicts, got %+v", c)
		}
	}
}
