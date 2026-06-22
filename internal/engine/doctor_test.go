package engine

import (
	"bytes"
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

// harnessProject builds a tempProject whose base playbook declares a harness
// block (settings + guard-bash hook) — the C1 wiring surface. The fixture base
// playbook already carries a `harness:` block (for the e2e guard-materialization
// test), so this REPLACES that block rather than appending a second one — strict
// decoding (FR-SCHEMA-013) correctly rejects a duplicate top-level key, so the
// helper must produce a single, valid `harness:` section.
func harnessProject(t *testing.T) string {
	t.Helper()
	root := tempProject(t)
	base := filepath.Join(root, "config", "playbook.yml")
	data, err := os.ReadFile(base)
	if err != nil {
		t.Fatal(err)
	}
	// Drop the fixture's existing harness: block (to EOF) and write the one the
	// C1 wiring check needs: settings + the declared guard-bash hook.
	if i := bytes.Index(data, []byte("\nharness:")); i >= 0 {
		data = data[:i]
	}
	harness := "\nharness:\n  claude:\n    settings: claude/settings.json\n    hooks:\n      - guard-bash\n"
	if err := os.WriteFile(base, append(data, harness...), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// guardedSettings is a settings file carrying a real guard envelope: a deny
// floor and a registration for the declared guard-bash hook.
const guardedSettings = `{
  "permissions": {"deny": ["Bash(sudo:*)"]},
  "hooks": {"PreToolUse": [{"matcher": "Bash",
    "hooks": [{"type": "command", "command": "sh ~/.claude/hooks/guard-bash"}]}]}
}`

// TestDoctorFailsCosmeticsOnlySettings is the C1 regression test: a harness
// settings file that materializes fine but carries only cosmetics (statusLine,
// no deny floor, no hook registrations) must turn doctor red — presence checks
// all pass while the built container's agent runs with zero guardrails.
func TestDoctorFailsCosmeticsOnlySettings(t *testing.T) {
	root := harnessProject(t) // fixture settings.json is statusLine-only
	res, err := doctorImpl(DoctorOpts{PlaybookPath: filepath.Join(root, "loom.yml")},
		fakeRuntime{exists: true, running: true})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	c, ok := checksByName(res)["host:harness:claude:settings"]
	if !ok || c.OK {
		t.Errorf("cosmetics-only settings must fail the wiring check, got %+v", c)
	}
}

func TestDoctorPassesGuardedSettings(t *testing.T) {
	root := harnessProject(t)
	settings := filepath.Join(root, "config", "dotfiles", "claude", "settings.json")
	if err := os.WriteFile(settings, []byte(guardedSettings), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := doctorImpl(DoctorOpts{PlaybookPath: filepath.Join(root, "loom.yml")},
		fakeRuntime{exists: true, running: true})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	c, ok := checksByName(res)["host:harness:claude:settings"]
	if !ok || !c.OK {
		t.Errorf("deny floor + hook registration should pass, got %+v", c)
	}
}

// TestDoctorStagedHomeDrift: the staging dir is graded against the config
// source — stale or missing staged guardrails turn doctor red instead of
// waiting for a build to notice (C1: wiring must be verifiable read-only).
func TestDoctorStagedHomeDrift(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{exists: true, running: true,
		ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}}

	// Nothing staged yet → drift.
	res, err := doctorImpl(DoctorOpts{PlaybookPath: pbPath}, rt)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if c, ok := checksByName(res)["host:staged-home"]; !ok || c.OK {
		t.Errorf("unstaged home should fail, got %+v", c)
	}

	// Build stages it → in sync.
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
		t.Fatalf("build: %v", err)
	}
	res, _ = doctorImpl(DoctorOpts{PlaybookPath: pbPath}, rt)
	if c, ok := checksByName(res)["host:staged-home"]; !ok || !c.OK {
		t.Errorf("freshly staged home should pass, got %+v", c)
	}

	// Config source moves on → drift again.
	prompt := filepath.Join(root, "config", "dotfiles", "bash", "prompt.go.sh")
	if err := os.WriteFile(prompt, []byte("# changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, _ = doctorImpl(DoctorOpts{PlaybookPath: pbPath}, rt)
	if c, ok := checksByName(res)["host:staged-home"]; !ok || c.OK {
		t.Errorf("source ahead of staging should fail, got %+v", c)
	}
}

// TestDoctorContainerHomeWiring: a running container is graded on its home
// sentinel vs the staged digest — the staged guardrails count only once the
// sync put them IN the container (C1).
func TestDoctorContainerHomeWiring(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	build := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}}
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, build, fixedClock); err != nil {
		t.Fatalf("build: %v", err)
	}
	want := homeDigest(filepath.Join(root, ".loom", "home"))
	if want == "" {
		t.Fatal("fixture should stage home content")
	}

	synced := fakeRuntime{exists: true, running: true, homeSentinel: want}
	res, _ := doctorImpl(DoctorOpts{PlaybookPath: pbPath}, synced)
	if c, ok := checksByName(res)["container:home"]; !ok || !c.OK {
		t.Errorf("matching sentinel should pass, got %+v", c)
	}

	stale := fakeRuntime{exists: true, running: true, homeSentinel: "deadbeef"}
	res, _ = doctorImpl(DoctorOpts{PlaybookPath: pbPath}, stale)
	if c, ok := checksByName(res)["container:home"]; !ok || c.OK {
		t.Errorf("stale sentinel should fail, got %+v", c)
	}

	// Stopped: never started to ask — graded at the staging tier only.
	stopped := fakeRuntime{exists: true, running: false}
	res, _ = doctorImpl(DoctorOpts{PlaybookPath: pbPath}, stopped)
	if _, ok := checksByName(res)["container:home"]; ok {
		t.Error("stopped container must not get a container:home verdict")
	}
}

// TestDoctorGradesGitconfigIdentity (T16 PR 3): a playbook shipping `gitconfig`
// gets a host-tier identity check — complete [user] passes with the value
// surfaced; an incomplete one fails (in-container commits would sign as the
// image default, violating the TEAM commit-identity rule).
func TestDoctorGradesGitconfigIdentity(t *testing.T) {
	rt := fakeRuntime{exists: false}
	res, err := doctorImpl(DoctorOpts{PlaybookPath: testFixture}, rt)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	c, ok := checksByName(res)["host:gitconfig"]
	if !ok || !c.OK {
		t.Fatalf("host:gitconfig should pass for the fixture identity, got %+v", c)
	}
	if !strings.Contains(c.Detail, "0+fixture@users.noreply.github.com") {
		t.Errorf("detail should surface the declared email, got %q", c.Detail)
	}
}

// TestGitIdentityParsesUserSection pins the minimal gitconfig walk: [user]
// keys win, other sections are ignored, missing keys come back empty.
func TestGitIdentityParsesUserSection(t *testing.T) {
	name, email := gitIdentity([]byte("[core]\n\teditor = vi\n[user]\n\tname = A B\n\temail = a@noreply\n"))
	if name != "A B" || email != "a@noreply" {
		t.Errorf("got (%q, %q), want (A B, a@noreply)", name, email)
	}
	if n, e := gitIdentity([]byte("[user]\n\tname = OnlyName\n")); n != "OnlyName" || e != "" {
		t.Errorf("partial identity: got (%q, %q)", n, e)
	}
	if n, e := gitIdentity([]byte("[alias]\n\temail = not-an-identity\n")); n != "" || e != "" {
		t.Errorf("keys outside [user] must be ignored: got (%q, %q)", n, e)
	}
}
