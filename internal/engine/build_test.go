package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iVersatile/loom/internal/lock"
)

// tempProject copies the fixture project into a temp dir so build's writes
// (loom.lock, .loom/) never touch the committed testdata.
func tempProject(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS("../playbook/testdata/proj")); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func fixedClock() time.Time { return time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC) }

func buildProber() fakeProber {
	return fakeProber{"git": "2.43", "jq": "1.7", "go": "go1.26.4", "gopls": "v0.16", "claude-code": "1.2.3"}
}

func TestBuildWritesLockMaterializesAndAudits(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-loom-dev", Image: defaultBaseImage, Status: "created"}}

	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, buildProber(), rt, fixedClock)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if res.Result != "created" || !res.LockWritten {
		t.Errorf("result=%q lockWritten=%t, want created/true", res.Result, res.LockWritten)
	}

	// loom.lock written with resolved pins.
	if _, err := os.Stat(filepath.Join(root, "loom.lock")); err != nil {
		t.Errorf("loom.lock not written: %v", err)
	}

	// $HOME materialized (the survive-rebuild artifacts, Q1/Q2).
	home := filepath.Join(root, ".loom", "home")
	settings := filepath.Join(home, ".claude", "settings.json")
	statusline := filepath.Join(home, ".claude", "statusline.sh")
	prompt := filepath.Join(home, ".bashrc.d", "prompt.go.sh")
	for _, f := range []string{settings, statusline, prompt} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("expected materialized %s: %v", f, err)
		}
	}
	// Shell script is executable.
	if fi, err := os.Stat(statusline); err == nil && fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("statusline.sh should be executable, mode=%v", fi.Mode())
	}

	// Audit log has an entry per mutation.
	if logged := countLogLines(t, root); logged == 0 {
		t.Error("expected action-log entries after build")
	}
	if len(res.Actions) == 0 {
		t.Error("BuildResult.Actions should reference the logged entries")
	}
}

func TestBuildIdempotent(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-loom-dev", Image: defaultBaseImage, Status: "created"}}

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, buildProber(), rt, fixedClock); err != nil {
		t.Fatalf("first build: %v", err)
	}
	first := countLogLines(t, root)

	// Second build: lock unchanged, dotfiles unchanged, container now "exists".
	rt2 := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-loom-dev", Image: defaultBaseImage, Status: "exists"}}
	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, buildProber(), rt2, fixedClock)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if res.Result != "converged" || res.LockWritten {
		t.Errorf("re-build should converge with no lock write, got result=%q lockWritten=%t", res.Result, res.LockWritten)
	}
	if got := countLogLines(t, root); got != first {
		t.Errorf("idempotent re-build appended %d new audit entries, want 0", got-first)
	}
}

func TestBuildContainerErrorAfterLock(t *testing.T) {
	// If the container step fails (e.g. no docker), the lock + materialize are
	// still written — a recoverable, re-runnable state (SPEC-verbs cross-cutting).
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureErr: os.ErrPermission}

	_, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, buildProber(), rt, fixedClock)
	if err == nil {
		t.Fatal("expected container-step error")
	}
	if _, err := os.Stat(filepath.Join(root, "loom.lock")); err != nil {
		t.Errorf("lock should be written before the container step: %v", err)
	}
}

func TestBuildBaseImageOverride(t *testing.T) {
	t.Setenv("LOOM_BASE_IMAGE", "ghcr.io/iversatile/loom-base:bookworm-slim")
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Status: "created"}}

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, buildProber(), rt, fixedClock); err != nil {
		t.Fatalf("build: %v", err)
	}
	l, err := lock.Read(filepath.Join(root, "loom.lock"))
	if err != nil || l == nil {
		t.Fatalf("read lock: %v", err)
	}
	if l.BaseImage != "ghcr.io/iversatile/loom-base:bookworm-slim" {
		t.Errorf("lock base_image = %q, want the LOOM_BASE_IMAGE override", l.BaseImage)
	}
}

func TestBuildPinsBaseDigest(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{resolveDigest: "sha256:deadbeef", ensureInfo: ContainerInfo{Status: "created"}}

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, buildProber(), rt, fixedClock); err != nil {
		t.Fatalf("build: %v", err)
	}
	l, err := lock.Read(filepath.Join(root, "loom.lock"))
	if err != nil || l == nil {
		t.Fatalf("read lock: %v", err)
	}
	if l.BaseImage != "debian:bookworm-slim@sha256:deadbeef" {
		t.Errorf("lock base_image = %q, want the pinned manifest digest", l.BaseImage)
	}
}

func TestProvisionScriptCoversSources(t *testing.T) {
	tools := []ToolInstall{
		{Name: "git", Source: "apt"},
		{Name: "jq", Source: "apt"},
		{Name: "go", Source: "go-tarball"},
		{Name: "gopls", Source: "go-install"},
		{Name: "gitleaks", Source: "go-install"},
		{Name: "uv", Source: "uv-installer"},
	}
	s := provisionScript(tools)
	for _, want := range []string{
		"apt-get install", " jq", "go.dev/dl",
		"go install golang.org/x/tools/gopls@latest",
		"go install github.com/zricethezav/gitleaks/v8@latest",
		"astral.sh/uv/install.sh", "bashrc.d",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("provision script missing %q\n---\n%s", want, s)
		}
	}
}

// TestProvisionScriptResilience pins the constrained-VM hardening (ADR-0011/0012):
// flaky steps are retried and the memory-heavy apt + Go steps are bounded so a
// ~7GB CI box (or a small Docker Desktop VM) survives provisioning.
func TestProvisionScriptResilience(t *testing.T) {
	s := provisionScript([]ToolInstall{
		{Name: "git", Source: "apt"},
		{Name: "gopls", Source: "go-install"},
	})
	for _, want := range []string{
		"retry()",                         // the retry helper is defined
		"Acquire::Languages=none",         // trimmed apt cache build (the OOM step)
		"retry apt-get",                   // apt is retried
		"retry go install",                // go install is retried
		"GOMEMLIMIT=1GiB", "GOMAXPROCS=1", // bounded Go memory footprint
		"GOFLAGS=-p=1", // serialized compile
	} {
		if !strings.Contains(s, want) {
			t.Errorf("provision script missing resilience guard %q\n---\n%s", want, s)
		}
	}
}

// TestProvisionSentinelMatchesDigest pins the reconcile contract (ADR-0011): the
// script writes, as its last step, exactly the toolset digest the runtime compares
// on re-build — so "fully provisioned" is distinguishable from "interrupted".
func TestProvisionSentinelMatchesDigest(t *testing.T) {
	tools := []ToolInstall{
		{Name: "go", Source: "go-tarball"},
		{Name: "gopls", Source: "go-install"},
		{Name: "git", Source: "apt"},
	}
	d := toolsetDigest(tools)
	if d == "" {
		t.Fatal("digest empty for a non-empty tool set")
	}
	// Order-stable: a reordered playbook must not read as drift.
	if got := toolsetDigest([]ToolInstall{tools[2], tools[0], tools[1]}); got != d {
		t.Errorf("digest not order-stable: %q vs %q", got, d)
	}
	if toolsetDigest(nil) != "" {
		t.Error("empty tool set must yield an empty digest (nothing to provision)")
	}
	s := provisionScript(tools)
	if !strings.Contains(s, provisionSentinel) {
		t.Errorf("provision script must write the sentinel %q", provisionSentinel)
	}
	if !strings.Contains(s, d) {
		t.Errorf("sentinel must carry the toolset digest %q\n---\n%s", d, s)
	}
}

func TestNeedsReprovision(t *testing.T) {
	for _, c := range []struct {
		name       string
		have, want string
		reprov     bool
	}{
		{"missing sentinel (interrupted)", "", "abc", true},
		{"drifted tool set", "old", "abc", true},
		{"converged", "abc", "abc", false},
		{"nothing to provision", "abc", "", false},
	} {
		if got := needsReprovision(c.have, c.want); got != c.reprov {
			t.Errorf("%s: needsReprovision(%q,%q)=%t want %t", c.name, c.have, c.want, got, c.reprov)
		}
	}
}

func countLogLines(t *testing.T, root string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".loom", "actions.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}
