package engine

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/lock"
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
	teardownRecord  *teardownArgs     // when set, Teardown records what was asked
	probeVersions   map[string]string // canned in-container versions (T5)
	homeSentinel    string            // canned home-sync sentinel digest (T7/C1)
	running         bool              // canned container state (LL-012)
	runningErr      error
	execExit        int       // canned command exit code for Exec
	execErr         error     // canned transport error for Exec
	startErr        error     // canned error for Start
	execRecord      *execCall // when set, Start/Exec record into it
}

// teardownArgs captures what the teardown verb asked of the runtime — the
// engine contract is the ASK (tier + cleanState passthrough), not a removal
// the fake fabricates (phase-1 review F1).
type teardownArgs struct {
	name       string
	level      string
	cleanState bool
}

// execCall captures what the exec verb asked of the runtime, for contract
// assertions (argv, workdir, start-before-exec, tty).
type execCall struct {
	name    string
	argv    []string
	workdir string
	started bool
	tty     bool
}

func (r fakeRuntime) Exists(string) (bool, error) { return r.exists, r.err }

func (r fakeRuntime) ResolveBaseDigest(string) (string, error) {
	return r.resolveDigest, r.resolveErr
}

func (r fakeRuntime) Ensure(ContainerSpec) (ContainerInfo, error) {
	return r.ensureInfo, r.ensureErr
}

func (r fakeRuntime) Teardown(name, level string, cleanState bool, _ io.Writer) (Removed, error) {
	if r.teardownRecord != nil {
		r.teardownRecord.name, r.teardownRecord.level = name, level
		r.teardownRecord.cleanState = cleanState
	}
	return r.teardownRemoved, r.teardownErr
}

// Probe returns canned in-container versions (T5): present iff a version is set.
func (r fakeRuntime) Probe(_, binary string) (bool, string) {
	v, ok := r.probeVersions[binary]
	return ok, v
}

// HomeDigest returns the canned home sentinel ("" = absent/stopped).
func (r fakeRuntime) HomeDigest(string) string { return r.homeSentinel }

// Running reports the canned container state (LL-012: plan picks live probe
// vs lock fallback on this).
func (r fakeRuntime) Running(string) (bool, error) { return r.running, r.runningErr }

func (r fakeRuntime) Start(name string) error {
	if r.execRecord != nil {
		r.execRecord.started = true
	}
	return r.startErr
}

func (r fakeRuntime) Exec(name string, argv []string, workdir string, tty bool) (int, error) {
	if r.execRecord != nil {
		r.execRecord.name, r.execRecord.argv, r.execRecord.workdir = name, argv, workdir
		r.execRecord.tty = tty
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

// fixtureProbes is the full converged container reality for the fixture
// playbook: every declared tool AND the claude-code agent binary (F2: the
// agent set is a graded dimension like tools).
func fixtureProbes() map[string]string {
	return map[string]string{
		"git":    "2.43",
		"jq":     "1.7",
		"go":     "go1.26.4",
		"gopls":  "v0.16",
		"claude": "1.2.3",
	}
}

// builtProject runs build against a tempProject with the given probes, so plan
// can be graded against a genuinely converged host side (lock written, home
// staged) — verdict and action share every dimension (F2).
func builtProject(t *testing.T, probes map[string]string) (pbPath, root string) {
	t.Helper()
	root = tempProject(t)
	pbPath = filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}, probeVersions: probes}
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
		t.Fatalf("build: %v", err)
	}
	return pbPath, root
}

func TestPlanDriftAndConverged(t *testing.T) {
	// Nothing built → container create + EVERY declared tool and agent is an
	// install: with no container there is no environment to grade (LL-012 —
	// the host PATH must not be consulted). The host-side dimensions report
	// too: no lock, nothing staged.
	res, err := planImpl(PlanOpts{PlaybookPath: testFixture}, fakeRuntime{exists: false})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !res.Changed() {
		t.Fatal("plan should report changes (container + installs)")
	}
	if !hasCreate(res.Create, "container", "loom-dev") {
		t.Errorf("create = %+v, want container loom-dev", res.Create)
	}
	if !hasCreate(res.Create, "lockfile", "loom.lock") {
		t.Errorf("create = %+v, want lockfile loom.lock (nothing resolved yet)", res.Create)
	}
	for _, tool := range []string{"git", "jq", "go", "gopls", "agent:claude-code"} {
		if !hasInstall(res.Install, tool) {
			t.Errorf("install = %+v, want %s (no environment exists to grade)", res.Install, tool)
		}
	}

	// Built host side + everything present in the RUNNING container → fully
	// converged on EVERY dimension: tools, agents, lockfile, staged home,
	// home sync (F2).
	pbPath, root := builtProject(t, fixtureProbes())
	res2, err := planImpl(PlanOpts{PlaybookPath: pbPath}, fakeRuntime{
		exists: true, running: true,
		probeVersions: fixtureProbes(),
		homeSentinel:  homeDigest(filepath.Join(root, ".loom", "home")),
	})
	if err != nil {
		t.Fatalf("plan converged: %v", err)
	}
	if res2.Changed() {
		t.Errorf("plan should be converged, got create=%v install=%v", res2.Create, res2.Install)
	}
	for _, n := range []string{"go@1.26", "agent:claude-code", "lockfile", "home", "home-sync"} {
		if !slices.Contains(res2.Noop, n) {
			t.Errorf("noop = %v, want %s converged", res2.Noop, n)
		}
	}
}

// TestPlanGradesContainerNotHost pins LL-012 / FR-PLAN-003 directly: the
// running container has every declared tool; whatever the HOST has is
// irrelevant (there is no host prober left to consult). Before the fix, a
// host missing ripgrep-class tools made plan report installs while build
// said converged — verdict and action must agree on the tools dimension.
func TestPlanGradesContainerNotHost(t *testing.T) {
	pbPath, root := builtProject(t, fixtureProbes())
	sentinel := homeDigest(filepath.Join(root, ".loom", "home"))
	res, err := planImpl(PlanOpts{PlaybookPath: pbPath}, fakeRuntime{
		exists: true, running: true,
		probeVersions: fixtureProbes(),
		homeSentinel:  sentinel,
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(res.Install) != 0 || len(res.Create) != 0 {
		t.Errorf("converged container must yield zero work: install=%+v create=%+v", res.Install, res.Create)
	}

	// And a running container genuinely MISSING a tool is reported — live
	// drift detection stays (ADR-0011).
	missingGopls := fixtureProbes()
	delete(missingGopls, "gopls")
	res2, err := planImpl(PlanOpts{PlaybookPath: pbPath}, fakeRuntime{
		exists: true, running: true,
		probeVersions: missingGopls,
		homeSentinel:  sentinel,
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
"gopls":{"intent":"latest","resolved":"","source":"go"}},
"agents":{"claude-code":{"resolved":"1.2.3"}}}`
	if err := os.WriteFile(filepath.Join(dir, "loom.lock"), []byte(lockBody), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := planImpl(PlanOpts{PlaybookPath: filepath.Join(dir, "loom.yml")},
		fakeRuntime{exists: true, running: false})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	// git/jq/go pinned in the lock → noop; gopls empty in the lock → install;
	// the agent pinned in the lock → noop too (F2: same fallback truth).
	if !hasInstall(res.Install, "gopls") || len(res.Install) != 1 {
		t.Errorf("install = %+v, want exactly gopls (empty lock pin)", res.Install)
	}
	if !slices.Contains(res.Noop, "go@1.26") {
		t.Errorf("noop = %v, want go@1.26 (lock-pinned)", res.Noop)
	}
	if !slices.Contains(res.Noop, "agent:claude-code") {
		t.Errorf("noop = %v, want agent:claude-code (lock-pinned, never started to ask)", res.Noop)
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

func hasCreate(items []CreateItem, kind, name string) bool {
	for _, c := range items {
		if c.Kind == kind && c.Name == name {
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

// TestPlanGradesLockfileAndHomeDimensions (phase-1 review F2): plan grades
// the host-side dimensions build converges — the lockfile and the staged
// $HOME — not just container+tools. Built → noop; source moves on → drift.
func TestPlanGradesLockfileAndHomeDimensions(t *testing.T) {
	pbPath, root := builtProject(t, fixtureProbes())
	sentinel := homeDigest(filepath.Join(root, ".loom", "home"))
	converged := fakeRuntime{exists: true, running: true,
		probeVersions: fixtureProbes(), homeSentinel: sentinel}

	res, err := planImpl(PlanOpts{PlaybookPath: pbPath}, converged)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if res.Changed() {
		t.Fatalf("built project should be converged, got %+v", res)
	}

	// A config-source edit build hasn't staged yet → dotfile drift.
	prompt := filepath.Join(root, "config", "dotfiles", "bash", "prompt.go.sh")
	if err := os.WriteFile(prompt, []byte("# new prompt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err = planImpl(PlanOpts{PlaybookPath: pbPath}, converged)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !hasCreate(res.Create, "dotfile", "~/.bashrc.d/prompt.go.sh") {
		t.Errorf("create = %+v, want the stale staged dotfile flagged", res.Create)
	}

	// A lock build would rewrite (stale base image pin) → lockfile drift.
	// Stale the parsed field, never a substring of the default image: CI pins
	// LOOM_BASE_IMAGE to the ghcr mirror, so the built lock carries whatever
	// baseImage() resolved and a defaultBaseImage replace silently no-ops
	// (PR #92 integration red, LL-013 — the test must exercise what it
	// claims under every env pin).
	lockPath := filepath.Join(root, "loom.lock")
	lk, err := lock.Read(lockPath)
	if err != nil || lk == nil {
		t.Fatalf("read built lock: %v", err)
	}
	if lk.BaseImage == "debian:ancient" {
		t.Fatal("fixture lock already stale — staling would be a no-op")
	}
	lk.BaseImage = "debian:ancient"
	if err := lk.WriteFile(lockPath); err != nil {
		t.Fatal(err)
	}
	res, err = planImpl(PlanOpts{PlaybookPath: pbPath}, converged)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !hasCreate(res.Create, "lockfile", "loom.lock") {
		t.Errorf("create = %+v, want loom.lock flagged for rewrite", res.Create)
	}
}

// TestPlanGradesAgentDimension (F2): the declared agent set is graded like
// tools — a running container missing the agent binary is an install, never
// silently skipped while build would provision it.
func TestPlanGradesAgentDimension(t *testing.T) {
	pbPath, root := builtProject(t, fixtureProbes())
	noAgent := fixtureProbes()
	delete(noAgent, "claude")
	res, err := planImpl(PlanOpts{PlaybookPath: pbPath}, fakeRuntime{
		exists: true, running: true,
		probeVersions: noAgent,
		homeSentinel:  homeDigest(filepath.Join(root, ".loom", "home")),
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !hasInstall(res.Install, "agent:claude-code") {
		t.Errorf("install = %+v, want agent:claude-code (absent in container)", res.Install)
	}
}

// TestPlanGradesHomeSyncDimension (F2): a clean staging dir can still be
// ahead of the container (interrupted sync) — plan reports the re-sync build
// would do, against the sentinel of a RUNNING container only.
func TestPlanGradesHomeSyncDimension(t *testing.T) {
	pbPath, _ := builtProject(t, fixtureProbes())

	// Stale sentinel → home-sync work.
	res, err := planImpl(PlanOpts{PlaybookPath: pbPath}, fakeRuntime{
		exists: true, running: true,
		probeVersions: fixtureProbes(), homeSentinel: "deadbeef",
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !hasCreate(res.Create, "home-sync", "loom-dev") {
		t.Errorf("create = %+v, want home-sync (sentinel behind staging)", res.Create)
	}

	// Stopped container: the sentinel is unreadable and plan never starts one
	// to ask — no home-sync verdict either way (graded at the staging tier).
	res2, err := planImpl(PlanOpts{PlaybookPath: pbPath}, fakeRuntime{
		exists: true, running: false,
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if hasCreate(res2.Create, "home-sync", "loom-dev") || slices.Contains(res2.Noop, "home-sync") {
		t.Errorf("stopped container must get no home-sync verdict, got %+v / %v", res2.Create, res2.Noop)
	}
}
