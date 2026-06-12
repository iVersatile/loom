package engine

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeProber reports presence/version from a fixed map (hermetic: no exec).
type fakeProber map[string]string

func (f fakeProber) probe(binary string) (bool, string) {
	v, ok := f[binary]
	return ok, v
}

type fakeRuntime struct {
	exists          bool
	err             error
	resolveDigest   string
	resolveErr      error
	ensureInfo      ContainerInfo
	ensureErr       error
	teardownRemoved Removed
	teardownErr     error
	probeVersions   map[string]string // canned in-container versions (T5)
	running         bool              // canned container state (LL-012)
	runningErr      error
	execExit        int       // canned command exit code for Exec
	execErr         error     // canned transport error for Exec
	startErr        error     // canned error for Start
	execRecord      *execCall // when set, Start/Exec record into it
}

// execCall captures what the exec verb asked of the runtime, for contract
// assertions (argv, workdir, start-before-exec).
type execCall struct {
	name    string
	argv    []string
	workdir string
	started bool
}

func (r fakeRuntime) Exists(string) (bool, error) { return r.exists, r.err }

func (r fakeRuntime) ResolveBaseDigest(string) (string, error) {
	return r.resolveDigest, r.resolveErr
}

func (r fakeRuntime) Ensure(ContainerSpec) (ContainerInfo, error) {
	return r.ensureInfo, r.ensureErr
}

func (r fakeRuntime) Teardown(string, string, io.Writer) (Removed, error) {
	return r.teardownRemoved, r.teardownErr
}

// Probe returns canned in-container versions (T5): present iff a version is set.
func (r fakeRuntime) Probe(_, binary string) (bool, string) {
	v, ok := r.probeVersions[binary]
	return ok, v
}

// Running reports the canned container state (LL-012: plan picks live probe
// vs lock fallback on this).
func (r fakeRuntime) Running(string) (bool, error) { return r.running, r.runningErr }

func (r fakeRuntime) Start(name string) error {
	if r.execRecord != nil {
		r.execRecord.started = true
	}
	return r.startErr
}

func (r fakeRuntime) Exec(name string, argv []string, workdir string) (int, error) {
	if r.execRecord != nil {
		r.execRecord.name, r.execRecord.argv, r.execRecord.workdir = name, argv, workdir
	}
	return r.execExit, r.execErr
}

// The fixture playbook resolves to tools: git, jq, go@1.26, gopls.
const testFixture = "../playbook/testdata/proj/loom.yml"

func TestDetectComputesDrift(t *testing.T) {
	p := fakeProber{
		"git": "git version 2.43",
		"jq":  "jq-1.7",
		"go":  "go version go1.26.4 linux/arm64", // satisfies go@1.26
		// gopls intentionally absent → drift
	}
	res, err := detectImpl(DetectOpts{PlaybookPath: testFixture}, p)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(res.Projects) != 1 || res.Projects[0].Name != "loom" || res.Projects[0].Stack != "go" {
		t.Errorf("projects = %+v, want one loom/go project", res.Projects)
	}
	// Only gopls should drift (absent); go satisfies via substring match.
	if len(res.Drift) != 1 || res.Drift[0].Tool != "gopls" || res.Drift[0].Have != nil {
		t.Errorf("drift = %+v, want only gopls absent", res.Drift)
	}
}

func TestPlanDriftAndConverged(t *testing.T) {
	// Nothing built → container create + EVERY declared tool is an install:
	// with no container there is no environment to grade (LL-012 — the host
	// PATH must not be consulted).
	res, err := planImpl(PlanOpts{PlaybookPath: testFixture}, fakeRuntime{exists: false})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !res.Changed() {
		t.Fatal("plan should report changes (container + installs)")
	}
	if len(res.Create) != 1 || res.Create[0].Name != "loom-dev" {
		t.Errorf("create = %+v, want container loom-dev", res.Create)
	}
	for _, tool := range []string{"git", "jq", "go", "gopls"} {
		if !hasInstall(res.Install, tool) {
			t.Errorf("install = %+v, want %s (no environment exists to grade)", res.Install, tool)
		}
	}

	// Everything present in the RUNNING container → fully converged.
	res2, err := planImpl(PlanOpts{PlaybookPath: testFixture}, fakeRuntime{
		exists: true, running: true,
		probeVersions: map[string]string{
			"git":   "2.43",
			"jq":    "1.7",
			"go":    "go1.26.4",
			"gopls": "v0.16",
		},
	})
	if err != nil {
		t.Fatalf("plan converged: %v", err)
	}
	if res2.Changed() {
		t.Errorf("plan should be converged, got create=%v install=%v", res2.Create, res2.Install)
	}
	if !slices.Contains(res2.Noop, "go@1.26") {
		t.Errorf("noop = %v, want go@1.26 converged", res2.Noop)
	}
}

// TestPlanGradesContainerNotHost pins LL-012 / FR-PLAN-003 directly: the
// running container has every declared tool; whatever the HOST has is
// irrelevant (there is no host prober left to consult). Before the fix, a
// host missing ripgrep-class tools made plan report installs while build
// said converged — verdict and action must agree on the tools dimension.
func TestPlanGradesContainerNotHost(t *testing.T) {
	res, err := planImpl(PlanOpts{PlaybookPath: testFixture}, fakeRuntime{
		exists: true, running: true,
		probeVersions: map[string]string{
			"git": "2.43", "jq": "1.7", "go": "go1.26.4", "gopls": "v0.16",
		},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(res.Install) != 0 || len(res.Create) != 0 {
		t.Errorf("converged container must yield zero work: install=%+v create=%+v", res.Install, res.Create)
	}

	// And a running container genuinely MISSING a tool is reported — live
	// drift detection stays (ADR-0011).
	res2, err := planImpl(PlanOpts{PlaybookPath: testFixture}, fakeRuntime{
		exists: true, running: true,
		probeVersions: map[string]string{"git": "2.43", "jq": "1.7", "go": "go1.26.4"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !hasInstall(res2.Install, "gopls") {
		t.Errorf("install = %+v, want gopls (genuinely absent in container)", res2.Install)
	}
}

// TestPlanStoppedContainerUsesLock: plan never mutates, so it must not Start
// a stopped container to probe it — the lock's container-pinned `resolved`
// values (T5) are the fallback truth.
func TestPlanStoppedContainerUsesLock(t *testing.T) {
	dir := tempProject(t)
	lockBody := `{"loom_lock":1,"resolved_at":"2026-06-11T00:00:00Z","base_image":"debian@sha256:x",
"tools":{"git":{"intent":"latest","resolved":"2.43","source":"apt"},
"jq":{"intent":"latest","resolved":"1.7","source":"apt"},
"go":{"intent":"1.26","resolved":"go1.26.4","source":"image"},
"gopls":{"intent":"latest","resolved":"","source":"go"}},"agents":{}}`
	if err := os.WriteFile(filepath.Join(dir, "loom.lock"), []byte(lockBody), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := planImpl(PlanOpts{PlaybookPath: filepath.Join(dir, "loom.yml")},
		fakeRuntime{exists: true, running: false})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	// git/jq/go pinned in the lock → noop; gopls empty in the lock → install.
	if !hasInstall(res.Install, "gopls") || len(res.Install) != 1 {
		t.Errorf("install = %+v, want exactly gopls (empty lock pin)", res.Install)
	}
	if !slices.Contains(res.Noop, "go@1.26") {
		t.Errorf("noop = %v, want go@1.26 (lock-pinned)", res.Noop)
	}
}

func TestPlanNoPlaybookErrors(t *testing.T) {
	_, err := planImpl(PlanOpts{PlaybookPath: "testdata/nope.yml"}, fakeRuntime{})
	if err == nil {
		t.Error("plan without a playbook should error")
	}
}

func hasInstall(items []InstallItem, tool string) bool {
	for _, i := range items {
		if i.Tool == tool {
			return true
		}
	}
	return false
}

// TestPlanHumanNamesTarget: the human verdict names the container it grades
// (guided-run finding ⑤ — a target-less verdict can't be safety-checked
// without --json).
func TestPlanHumanNamesTarget(t *testing.T) {
	res, err := planImpl(PlanOpts{PlaybookPath: testFixture}, fakeRuntime{exists: false})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if want := containerName("loom"); !strings.Contains(res.Human(), want) {
		t.Errorf("plan human line should name %q, got %q", want, res.Human())
	}
}
