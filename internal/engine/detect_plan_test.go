package engine

import (
	"slices"
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
}

func (r fakeRuntime) Exists(string) (bool, error) { return r.exists, r.err }

func (r fakeRuntime) ResolveBaseDigest(string) (string, error) {
	return r.resolveDigest, r.resolveErr
}

func (r fakeRuntime) Ensure(ContainerSpec) (ContainerInfo, error) {
	return r.ensureInfo, r.ensureErr
}

func (r fakeRuntime) Teardown(string, string) (Removed, error) {
	return r.teardownRemoved, r.teardownErr
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
	// Nothing built, gopls missing → container create + gopls install, drift.
	p := fakeProber{
		"git": "git version 2.43",
		"jq":  "jq-1.7",
		"go":  "go version go1.26.4",
	}
	res, err := planImpl(PlanOpts{PlaybookPath: testFixture}, p, fakeRuntime{exists: false})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !res.Changed() {
		t.Fatal("plan should report changes (container + gopls)")
	}
	if len(res.Create) != 1 || res.Create[0].Name != "loom-loom-dev" {
		t.Errorf("create = %+v, want container loom-loom-dev", res.Create)
	}
	if !hasInstall(res.Install, "gopls") {
		t.Errorf("install = %+v, want gopls", res.Install)
	}
	if !slices.Contains(res.Noop, "go@1.26") {
		t.Errorf("noop = %v, want go@1.26 converged", res.Noop)
	}

	// Everything present + container exists → fully converged.
	pAll := fakeProber{
		"git":   "2.43",
		"jq":    "1.7",
		"go":    "go1.26.4",
		"gopls": "v0.16",
	}
	res2, err := planImpl(PlanOpts{PlaybookPath: testFixture}, pAll, fakeRuntime{exists: true})
	if err != nil {
		t.Fatalf("plan converged: %v", err)
	}
	if res2.Changed() {
		t.Errorf("plan should be converged, got create=%v install=%v", res2.Create, res2.Install)
	}
}

func TestPlanNoPlaybookErrors(t *testing.T) {
	_, err := planImpl(PlanOpts{PlaybookPath: "testdata/nope.yml"}, fakeProber{}, fakeRuntime{})
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
