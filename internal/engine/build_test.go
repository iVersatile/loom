package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iVersatile/loom/internal/lock"
	"github.com/iVersatile/loom/internal/playbook"
	"github.com/iVersatile/loom/internal/resolver"
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

// guardsOnlyPlaybook is the lighter loom.yml tempGuardsProject writes — the shared
// fixture (above) MINUS the Go toolchain (`stack: go` and its go@1.26/gopls/
// go/strict). The role-deny guards are declared in config/playbook.yml's harness:
// block, which is independent of the stack, so they still materialize. Everything
// else (base image, loom overlay, ports, env, ci, config_source) is unchanged so
// the built container still resembles the real guard deployment.
const guardsOnlyPlaybook = `loom: 1
tier: project
name: loom
extends: base
overlay: loom
ports:
  - 8080
env:
  - ANTHROPIC_API_KEY
ci:
  - ci
config_source:
  type: local
  path: ./config
`

// tempGuardsProject is a LIGHTER variant of tempProject for the guards e2e
// (TestE2EGuardsBlockByRole, FR-GUARD-E2E): it copies the shared fixture, then
// overwrites loom.yml to DROP the Go toolchain. The guards e2e only needs the
// role-deny guards materialized — it never compiles Go — so provisioning the
// go@1.26 toolchain (a tarball download + `go install gopls`) was pure weight on
// the e2e build container, inflating the container-cgroup memory that OOM-kills
// (exit 137) the CI integration gate mid-provisioning (#75 mitigation #2). The
// shared tempProject stays heavy on purpose: the toolchain-resolution tests
// (detect_plan/doctor/build) depend on go@1.26 in that fixture.
func tempGuardsProject(t *testing.T) string {
	t.Helper()
	root := tempProject(t)
	if err := os.WriteFile(filepath.Join(root, "loom.yml"), []byte(guardsOnlyPlaybook), 0o644); err != nil {
		t.Fatalf("write guards-only loom.yml: %v", err)
	}
	return root
}

func fixedClock() time.Time { return time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC) }

func TestBuildWritesLockMaterializesAndAudits(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"}}

	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock)
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
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"}}

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
		t.Fatalf("first build: %v", err)
	}
	first := countLogLines(t, root)

	// Second build: lock unchanged, dotfiles unchanged, container now "exists".
	rt2 := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "exists"}}
	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt2, fixedClock)
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

	_, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock)
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

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
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
	// Hermetic: this asserts digest-pinning of the DEFAULT base, so clear any
	// ambient LOOM_BASE_IMAGE (the integration job sets it for the ghcr e2e, which
	// otherwise leaks into this unit test and changes the expected image) — LL-006.
	t.Setenv("LOOM_BASE_IMAGE", "")
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{resolveDigest: "sha256:deadbeef", ensureInfo: ContainerInfo{Status: "created"}}

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
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
		{Name: "golangci-lint", Source: "go-install"},
		{Name: "uv", Source: "uv-installer"},
		{Name: "nodejs", Source: "nodejs-20"},
	}
	s := provisionScript(tools, nil)
	for _, want := range []string{
		"apt-get install", " jq", "go.dev/dl",
		"go install golang.org/x/tools/gopls@latest",
		"go install github.com/zricethezav/gitleaks/v8@latest",
		"go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest",
		"astral.sh/uv/install.sh",
		// nodejs MUST come from the NodeSource setup_20.x script, NOT apt (Node 18) —
		// the gemini-cli Node>=20 prerequisite (B1).
		"deb.nodesource.com/setup_20.x",
		// adv-065: go-built tools, uv, and the harness all land in the SHARED
		// /usr/local/bin (reachable by the non-root runtime user), not root's home.
		"export GOBIN=/usr/local/bin",
		"UV_INSTALL_DIR=/usr/local/bin",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("provision script missing %q\n---\n%s", want, s)
		}
	}
	// T4: shell config has ONE owner. The provision script never appends to
	// shell-init files (persistent PATH lives in ~/.bashrc.d/path.go.sh; the
	// loader is ensureShellInit's, unconditional) and never installs into root's
	// home in a way a non-root user cannot reach (adv-065: GOBIN moves go-built
	// tools off /root/go/bin).
	for _, never := range []string{".profile", ".bashrc", "/root/go/bin"} {
		if strings.Contains(s, never) {
			t.Errorf("provision script must not touch %q (T4 single owner)\n---\n%s", never, s)
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
	}, nil)
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
	d := provisionDigest(tools, nil)
	if d == "" {
		t.Fatal("digest empty for a non-empty tool set")
	}
	// Order-stable: a reordered playbook must not read as drift.
	if got := provisionDigest([]ToolInstall{tools[2], tools[0], tools[1]}, nil); got != d {
		t.Errorf("digest not order-stable: %q vs %q", got, d)
	}
	if provisionDigest(nil, nil) != "" {
		t.Error("empty tool set must yield an empty digest (nothing to provision)")
	}
	s := provisionScript(tools, nil)
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

// TestHomeDigestDetectsDotfileChange pins the T7 fix: the home staging digest is
// stable for identical content and changes when any staged file's content
// changes or a file is added — the trigger that re-syncs an existing
// container's $HOME on a dotfile-only build (presence != convergence,
// ADR-0011/ADR-0015).
func TestHomeDigestDetectsDotfileChange(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude/statusline.sh", "echo v1\n")
	write(".bashrc.d/prompt.go.sh", "PS1=go\n")

	d1 := homeDigest(dir)
	if d1 == "" {
		t.Fatal("digest empty for a non-empty staging dir")
	}
	if d2 := homeDigest(dir); d2 != d1 {
		t.Errorf("digest not stable: %q vs %q", d2, d1)
	}
	// Content change → new digest (the T7 trigger).
	write(".claude/statusline.sh", "echo v2\n")
	changed := homeDigest(dir)
	if changed == d1 {
		t.Error("content change must change the home digest")
	}
	// New file → new digest.
	write(".claude/settings.json", "{}\n")
	if homeDigest(dir) == changed {
		t.Error("added file must change the home digest")
	}
	// Empty/missing staging ⇒ "" (nothing to sync).
	if homeDigest(t.TempDir()) != "" {
		t.Error("empty staging dir must yield an empty digest")
	}
	if homeDigest(filepath.Join(dir, "no-such-dir")) != "" {
		t.Error("missing staging dir must yield an empty digest")
	}
}

func TestNeedsHomeSync(t *testing.T) {
	for _, c := range []struct {
		name       string
		have, want string
		sync       bool
	}{
		{"missing sentinel (pre-T7 container or interrupted cp)", "", "abc", true},
		{"drifted home (dotfile-only change)", "old", "abc", true},
		{"converged", "abc", "abc", false},
		{"nothing staged", "abc", "", false},
	} {
		if got := needsHomeSync(c.have, c.want); got != c.sync {
			t.Errorf("%s: needsHomeSync(%q,%q)=%t want %t", c.name, c.have, c.want, got, c.sync)
		}
	}
}

// TestProvisionScriptInstallsAgent pins T8 + adv-065: a declared agent yields its
// native install step (claude-code, no Node), THEN relocates the binary from
// root's ~/.local/bin to the SHARED /usr/local/bin so the non-root runtime user
// can run it. Persistent PATH for /usr/local/bin is the default login PATH, never
// an append to shell-init files here (T4).
func TestProvisionScriptInstallsAgent(t *testing.T) {
	s := provisionScript(nil, []AgentInstall{{Name: "claude-code", Source: "native-installer"}})
	for _, want := range []string{
		"claude.ai/install.sh", // the native installer is invoked
		"retry ",               // wrapped in the resilience retry helper
		`cp -L "$HOME/.local/bin/claude" /usr/local/bin/claude`, // relocated to the shared bin
		"chmod 0755 /usr/local/bin/claude",                      // world-exec for the non-root user
	} {
		if !strings.Contains(s, want) {
			t.Errorf("agent provision missing %q\n---\n%s", want, s)
		}
	}
	for _, never := range []string{".profile", ".bashrc"} {
		if strings.Contains(s, never) {
			t.Errorf("agent provision must not touch %q (T4: PATH is dotfile-owned)\n---\n%s", never, s)
		}
	}
	// An unknown agent emits no install step (recorded in the digest, not installed).
	if got := provisionScript(nil, []AgentInstall{{Name: "mystery", Source: ""}}); strings.Contains(got, "install.sh") {
		t.Errorf("unknown agent should emit no installer\n---\n%s", got)
	}
}

// TestShellInitWiresBothShells pins T4's converged shell-init: ONE loader,
// written to BOTH login (.profile) and interactive (.bashrc) init files, so a
// dotfile-set PATH applies regardless of how the shell is invoked. $HOME-based
// (T10 prep) and grep-guarded (idempotent across rebuilds and across the
// loader line older builds appended).
func TestShellInitWiresBothShells(t *testing.T) {
	for _, want := range []string{
		`"$HOME/.profile"`, // login shells load the dotfile dir
		`"$HOME/.bashrc"`,  // interactive shells load the same dir
		".bashrc.d",        // the one owning directory
		"grep -qs",         // idempotence guard
		"mkdir -p",         // dir exists even for a dotfile-less playbook
	} {
		if !strings.Contains(shellInitScript, want) {
			t.Errorf("shell-init missing %q\n---\n%s", want, shellInitScript)
		}
	}
	if strings.Contains(shellInitScript, "/root/") {
		t.Errorf("shell-init must use $HOME, never /root (T10 prep)\n---\n%s", shellInitScript)
	}
}

// TestProvisionDigestCoversAgents pins that the reconcile sentinel reflects agents,
// so adding/removing an agent re-provisions an existing container (T8 + T7 trigger).
func TestProvisionDigestCoversAgents(t *testing.T) {
	tools := []ToolInstall{{Name: "go", Source: "go-tarball"}}
	bare := provisionDigest(tools, nil)
	withAgent := provisionDigest(tools, []AgentInstall{{Name: "claude-code", Source: "native-installer"}})
	if bare == withAgent {
		t.Error("adding an agent must change the provision digest")
	}
	// Order-stable across agents.
	a := []AgentInstall{{Name: "claude-code", Source: "native-installer"}, {Name: "codex", Source: ""}}
	if provisionDigest(tools, a) != provisionDigest(tools, []AgentInstall{a[1], a[0]}) {
		t.Error("agent digest not order-stable")
	}
	if provisionDigest(nil, nil) != "" {
		t.Error("empty tools+agents must yield an empty digest")
	}
}

// TestAgentInstalls pins the resolution→install mapping: every resolved agent is
// emitted, sorted, with claude-code carrying the native-installer source.
func TestAgentInstalls(t *testing.T) {
	r := &resolver.Resolution{Agents: map[string]lock.LockedAgent{
		"codex":       {},
		"claude-code": {},
	}}
	got := agentInstalls(r)
	if len(got) != 2 || got[0].Name != "claude-code" || got[1].Name != "codex" {
		t.Fatalf("agentInstalls not sorted/complete: %+v", got)
	}
	if got[0].Source != "native-installer" {
		t.Errorf("claude-code source = %q, want native-installer", got[0].Source)
	}
	if got[1].Source != "" {
		t.Errorf("unknown agent should have empty source, got %q", got[1].Source)
	}
}

// TestEnvArgs pins the creds mechanism: declared env var NAMES become `-e NAME`
// passthrough args (value forwarded by docker from loom's env; never embedded).
func TestEnvArgs(t *testing.T) {
	got := envArgs([]string{"ANTHROPIC_API_KEY", "", "CLAUDE_CODE_OAUTH_TOKEN"})
	want := []string{"-e", "ANTHROPIC_API_KEY", "-e", "CLAUDE_CODE_OAUTH_TOKEN"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("envArgs = %v, want %v", got, want)
	}
	// No value is ever embedded (only the bare name follows -e).
	for _, a := range got {
		if strings.Contains(a, "=") {
			t.Errorf("env arg %q must not carry a value", a)
		}
	}
	if len(envArgs(nil)) != 0 {
		t.Error("no declared env → no args")
	}
}

// TestCredsMount pins the creds-reuse path (T8/ADR-0014): mount the host's
// existing ~/.claude/.credentials.json read-only, only when claude-code is being
// installed and the file is present — never mount a missing path.
func TestCredsMount(t *testing.T) {
	claude := []AgentInstall{{Name: "claude-code", Source: "native-installer"}}

	got := credsMount("/host/.claude/.credentials.json", true, claude, containerHome)
	want := []string{"-v", "/host/.claude/.credentials.json:" + containerHome + "/.claude/.credentials.json:ro"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("credsMount = %v, want %v", got, want)
	}
	// Read-only, single file.
	if !strings.HasSuffix(got[1], ":ro") {
		t.Errorf("creds mount must be read-only: %q", got[1])
	}
	// T10: the mount targets the RESOLVED home — a non-root user's creds land in
	// /home/<user>/.claude, not /root.
	if got := credsMount("/host/.claude/.credentials.json", true, claude, "/home/dev"); got[1] != "/host/.claude/.credentials.json:/home/dev/.claude/.credentials.json:ro" {
		t.Errorf("creds mount must follow the resolved home: %q", got[1])
	}
	// No mount when the file is absent (would make docker create a dir).
	if credsMount("/host/.claude/.credentials.json", false, claude, containerHome) != nil {
		t.Error("absent creds file must not be mounted")
	}
	// No mount when claude-code is not among the agents.
	if credsMount("/host/.claude/.credentials.json", true, []AgentInstall{{Name: "codex"}}, containerHome) != nil {
		t.Error("creds mount only applies when claude-code is installed")
	}
}

// TestBuildLockRecordsContainerVersions pins the T5 fix: the lock's `resolved`
// comes from probing INSIDE the converged container, never the build host — a
// Mac build must not record "Apple Git" for a debian container. A binary the
// container lacks stays "" (the not-found case is honest, not swallowed).
func TestBuildLockRecordsContainerVersions(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{
		ensureInfo: ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
		probeVersions: map[string]string{
			"git":    "git version 2.39.5 (debian)",
			"jq":     "jq-1.7.1",
			"go":     "go version go1.26.4 linux/arm64",
			"claude": "1.0.35 (Claude Code)", // probed via the claude-code -> claude alias
		}, // gopls deliberately absent from the container
	}

	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	l, err := lock.Read(filepath.Join(root, "loom.lock"))
	if err != nil || l == nil {
		t.Fatalf("read lock: %v", err)
	}
	if got := l.Tools["git"].Resolved; got != "git version 2.39.5 (debian)" {
		t.Errorf("git resolved = %q, want the container-probed version", got)
	}
	if got := l.Tools["go"].Resolved; !strings.Contains(got, "linux") {
		t.Errorf("go resolved = %q, want the container's (linux) go", got)
	}
	if got := l.Tools["gopls"].Resolved; got != "" {
		t.Errorf("gopls resolved = %q, want \"\" (absent from container, never host-filled)", got)
	}
	if got := res.Resolved["git"].Resolved; got != "git version 2.39.5 (debian)" {
		t.Errorf("result resolved git = %q, want container value", got)
	}
	if got := l.Agents["claude-code"].Resolved; got != "1.0.35 (Claude Code)" {
		t.Errorf("claude-code resolved = %q, want the in-container `claude` version (binary alias)", got)
	}

	// Idempotent: a second build against the same container re-probes the same
	// versions and must not rewrite the lock.
	rt2 := rt
	rt2.ensureInfo.Status = "exists"
	res2, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt2, fixedClock)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if res2.LockWritten {
		t.Error("unchanged container versions must not rewrite the lock")
	}
}

// TestContainerName pins the naming convention (T11): `<project>-dev`, no
// `loom-` prefix — the loom-managed marker is the labels, not the name.
func TestContainerName(t *testing.T) {
	if got := containerName("loom"); got != "loom-dev" {
		t.Errorf("containerName(loom) = %q, want loom-dev", got)
	}
	if got := containerName("prompiler"); got != "prompiler-dev" {
		t.Errorf("containerName(prompiler) = %q, want prompiler-dev", got)
	}
}

// TestCreateRunArgs pins the create-time container surface: managed labels
// (T11), the durable agent-home volume at ~/.claude (T14), the RW project
// bind-mount (T13), and the optional host creds file — base image and command
// last. All create-time-only: changing any of it requires --force.
func TestCreateRunArgs(t *testing.T) {
	spec := ContainerSpec{
		Name: "loom-dev", Project: "loom", BaseImage: "debian:bookworm-slim",
		Agents:     []AgentInstall{{Name: "claude-code", Source: "native-installer"}},
		Tools:      []ToolInstall{{Name: "gh", Source: "go-install"}},
		ProjectDir: "/host/repo/loom",
	}
	got := strings.Join(createRunArgs(spec, "/host/.claude/.credentials.json", true), " ")

	for _, want := range []string{
		"--label loom.managed=true",
		"--label loom.project=loom",
		"-v loom-dev-claude:" + containerHome + "/.claude",
		// gh-config volume (ADR-0026): persists `gh auth login` across rebuilds,
		// mounted only because gh is a declared tool.
		"-v loom-dev-gh:" + containerHome + "/.config/gh",
		"-v /host/repo/loom:/workspace/loom",
		"-v /host/.claude/.credentials.json:" + containerHome + "/.claude/.credentials.json:ro",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("createRunArgs missing %q in: %s", want, got)
		}
	}
	if !strings.HasSuffix(got, "debian:bookworm-slim sleep infinity") {
		t.Errorf("image+command must come last: %s", got)
	}

	// No agent → no agent-home volume; no gh tool → no gh-config volume; no
	// project dir → no workspace mount.
	bare := ContainerSpec{Name: "x-dev", Project: "x", BaseImage: "img"}
	g := strings.Join(createRunArgs(bare, "", false), " ")
	if strings.Contains(g, "-claude:") || strings.Contains(g, "-gh:") || strings.Contains(g, "/workspace/") {
		t.Errorf("bare spec must not mount volume or workspace: %s", g)
	}

	// T10 PR 3 (Model A): a non-root user retargets the home mounts to
	// /home/<user>, and the container itself still runs as root — NO `--user`
	// on `docker run` (entry verbs carry `-u`, not the daemon).
	nonRoot := ContainerSpec{
		Name: "loom-dev", Project: "loom", BaseImage: "debian:bookworm-slim",
		Agents: []AgentInstall{{Name: "claude-code", Source: "native-installer"}},
		Tools:  []ToolInstall{{Name: "gh", Source: "go-install"}},
		User:   "dev", Home: "/home/dev",
	}
	gr := strings.Join(createRunArgs(nonRoot, "/host/.claude/.credentials.json", true), " ")
	if !strings.Contains(gr, "-v loom-dev-claude:/home/dev/.claude") {
		t.Errorf("non-root agent-home volume must target /home/dev: %s", gr)
	}
	if !strings.Contains(gr, "-v loom-dev-gh:/home/dev/.config/gh") {
		t.Errorf("non-root gh-config volume must target /home/dev: %s", gr)
	}
	if !strings.Contains(gr, "/home/dev/.claude/.credentials.json:ro") {
		t.Errorf("non-root creds mount must target /home/dev: %s", gr)
	}
	if strings.Contains(gr, "--user") {
		t.Errorf("Model A: docker run must NOT set --user (entry verbs do): %s", gr)
	}
}

// TestHomeCpTargetSingleOwner (T10 PR 1): every in-container home path must
// derive from the containerHome constant — ADR-0016's "T10 retargets entry by
// changing one configured-user value" only holds if nothing bypasses it. Two
// literal ":/root/" cp targets were found doing exactly that at T10 design
// time; this pins the helper AND greps the source so a third can't return.
func TestHomeCpTargetSingleOwner(t *testing.T) {
	if got, want := homeCpTarget("loom-dev", containerHome), "loom-dev:"+containerHome+"/"; got != want {
		t.Errorf("homeCpTarget = %q, want %q", got, want)
	}
	// T10 PR 3: a non-root home retargets the cp destination.
	if got, want := homeCpTarget("loom-dev", "/home/dev"), "loom-dev:/home/dev/"; got != want {
		t.Errorf("homeCpTarget(non-root) = %q, want %q", got, want)
	}
	src, err := os.ReadFile("container.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), `":/root/"`); n > 0 {
		t.Errorf("container.go has %d literal \":/root/\" bypass(es) of containerHome (T10 single-owner rule)", n)
	}
}

// TestHomeForUserResolution (T10 PR 2, FR-SCHEMA-009): the resolved container
// $HOME is /root for the default (unset or explicit root) — so every pre-T10
// playbook keeps its exact home — and /home/<user> for any non-root user.
func TestHomeForUserResolution(t *testing.T) {
	cases := map[string]string{
		"":      containerHome, // unset = root (Phase-1 default)
		"root":  containerHome, // explicit root = the default, not /home/root
		"dev":   "/home/dev",
		"agent": "/home/agent",
	}
	for user, want := range cases {
		if got := homeForUser(user); got != want {
			t.Errorf("homeForUser(%q) = %q, want %q", user, got, want)
		}
	}
}

// TestBuildPopulatesUserAndHome (T10 PR 2, FR-SCHEMA-009): build plumbs the
// merged user: value and its resolved $HOME onto the ContainerSpec it converges.
// PR 2 only LAYS this value; the engine consumes it (docker run --user, home
// retarget, chown) in PR 3 — so this asserts the plumbing, not the behavior.
func TestBuildPopulatesUserAndHome(t *testing.T) {
	t.Setenv("LOOM_SESSION_ROLE", "") // hermetic: this test exercises user/home plumbing, not role resolution
	// Default: the fixture sets no user:, so the spec stays root-homed.
	root := tempProject(t)
	var spec ContainerSpec
	rt := fakeRuntime{
		ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
		ensureRecord: &spec,
	}
	if _, err := buildImpl(BuildOpts{PlaybookPath: filepath.Join(root, "loom.yml")}, rt, fixedClock); err != nil {
		t.Fatalf("build (default): %v", err)
	}
	if spec.User != "" || spec.Home != containerHome {
		t.Errorf("default: User=%q Home=%q, want \"\"/%q", spec.User, spec.Home, containerHome)
	}

	// user: set on the project overlay flows through merge → spec, $HOME resolved.
	// A non-root user requires a role: (adv-067 TASK 2: an empty marker breaks the
	// drain role-guard), so declare one — this test exercises user/home plumbing.
	root2 := tempProject(t)
	pbPath := filepath.Join(root2, "loom.yml")
	data, err := os.ReadFile(pbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pbPath, append(data, []byte("\nuser: agent\nrole: agent\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	var spec2 ContainerSpec
	rt2 := fakeRuntime{
		ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
		ensureRecord: &spec2,
	}
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt2, fixedClock); err != nil {
		t.Fatalf("build (user set): %v", err)
	}
	if spec2.User != "agent" || spec2.Home != "/home/agent" {
		t.Errorf("user set: User=%q Home=%q, want agent//home/agent", spec2.User, spec2.Home)
	}
}

// TestBuildPopulatesNoEgress covers FR-NET-001 (T20 S2a, ADR-0028): a resolved
// playbook declaring networking.egress: none plumbs NoEgress: true onto the
// ContainerSpec build converges (which createRunArgs realizes as --network none —
// the S1 mechanism); off/unset leave NoEgress false (full egress, Phase-1
// default). This is the engine seam — the single behavioral mapping in the slice —
// proven WITHOUT docker (the live --network none cut is the integration canary).
func TestBuildPopulatesNoEgress(t *testing.T) {
	t.Setenv("LOOM_SESSION_ROLE", "") // hermetic: this test exercises egress plumbing, not role resolution

	// Default: the fixture declares no networking:, so the spec stays full-egress.
	root := tempProject(t)
	var spec ContainerSpec
	rt := fakeRuntime{
		ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
		ensureRecord: &spec,
	}
	if _, err := buildImpl(BuildOpts{PlaybookPath: filepath.Join(root, "loom.yml")}, rt, fixedClock); err != nil {
		t.Fatalf("build (default): %v", err)
	}
	if spec.NoEgress {
		t.Errorf("default: NoEgress=true, want false (unset networking = off = full egress)")
	}

	// networking.egress: none on the project overlay flows through merge → spec.
	root2 := tempProject(t)
	pbPath := filepath.Join(root2, "loom.yml")
	data, err := os.ReadFile(pbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pbPath, append(data, []byte("\nnetworking:\n  egress: none\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	var spec2 ContainerSpec
	rt2 := fakeRuntime{
		ensureInfo:   ContainerInfo{Name: "loom-dev", Image: defaultBaseImage, Status: "created"},
		ensureRecord: &spec2,
	}
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt2, fixedClock); err != nil {
		t.Fatalf("build (egress none): %v", err)
	}
	if !spec2.NoEgress {
		t.Errorf("egress none: NoEgress=false, want true (networking.egress: none → --network none)")
	}
}

// TestNoEgressMapping pins the pure posture→cut mapping (FR-NET-001): only
// egress: none turns the cut on; off, unset (nil networking), and an empty section
// leave it off (full egress, Phase-1 default).
func TestNoEgressMapping(t *testing.T) {
	cases := []struct {
		name string
		pb   playbook.Playbook
		want bool
	}{
		{"nil networking (unset = off)", playbook.Playbook{}, false},
		{"empty section (= off)", playbook.Playbook{Networking: &playbook.Networking{}}, false},
		{"explicit off", playbook.Playbook{Networking: &playbook.Networking{Egress: playbook.EgressOff}}, false},
		{"none cuts egress", playbook.Playbook{Networking: &playbook.Networking{Egress: playbook.EgressNone}}, true},
	}
	for _, c := range cases {
		if got := noEgress(&c.pb); got != c.want {
			t.Errorf("%s: noEgress = %t, want %t", c.name, got, c.want)
		}
	}
}

// TestCreateRunArgsNoEgress covers FR-NET-001 (the createRunArgs companion to the
// S1 test): NoEgress: true appends --network none; the default (false) does not.
// This is the create-time half of the egress cut — the live container proof is the
// integration canary.
func TestCreateRunArgsNoEgress(t *testing.T) {
	cut := ContainerSpec{Name: "x-dev", Project: "x", BaseImage: "img", NoEgress: true}
	if got := strings.Join(createRunArgs(cut, "", false), " "); !strings.Contains(got, "--network none") {
		t.Errorf("NoEgress spec must append --network none: %s", got)
	}
	open := ContainerSpec{Name: "x-dev", Project: "x", BaseImage: "img"}
	if got := strings.Join(createRunArgs(open, "", false), " "); strings.Contains(got, "--network") {
		t.Errorf("default spec must NOT set --network (full egress): %s", got)
	}
}

// TestExecUserArgs (T10 PR 3, Model A): entry verbs run AS the configured user
// by name; root/unset adds no flag.
func TestExecUserArgs(t *testing.T) {
	if got := execUserArgs(""); got != nil {
		t.Errorf("execUserArgs(\"\") = %v, want nil (container default)", got)
	}
	if got := execUserArgs("root"); got != nil {
		t.Errorf("execUserArgs(root) = %v, want nil (root is the default)", got)
	}
	if got := strings.Join(execUserArgs("dev"), " "); got != "-u dev" {
		t.Errorf("execUserArgs(dev) = %q, want \"-u dev\"", got)
	}
}

// TestProvisionUserScript (T10 PR 3, red-team finding 4): empty for root;
// idempotent + collision-tolerant for a real user — reuse on name-exists, next
// free uid (no -u) on a uid-1000 collision, uid 1000 only when free.
func TestProvisionUserScript(t *testing.T) {
	if s := provisionUserScript(""); s != "" {
		t.Errorf("root/unset must emit no useradd, got %q", s)
	}
	if s := provisionUserScript("root"); s != "" {
		t.Errorf("explicit root must emit no useradd, got %q", s)
	}
	s := provisionUserScript("dev")
	for _, want := range []string{
		"id -u dev",              // reuse guard (idempotent)
		"id -u 1000",             // collision probe
		"useradd -m dev",         // next-free path (NO -u) on collision
		"useradd -m -u 1000 dev", // preferred uid when free
		"auto-assigned uid",      // the collision is logged, not silent
	} {
		if !strings.Contains(s, want) {
			t.Errorf("provisionUserScript(dev) missing %q\n---\n%s", want, s)
		}
	}
	// Never a hard uid baked where it can't move: useradd pins 1000 ONLY on the
	// preferred-when-free branch (the other 1000 is the `id -u 1000` probe).
	if strings.Count(s, "useradd -m -u 1000") != 1 {
		t.Errorf("uid 1000 must be the preferred-when-free useradd only, not a hard pin:\n%s", s)
	}
}

// TestChownHomeScript (T10 PR 3, red-team finding 3): empty for root; for a user
// it chowns the resolved home but PRUNES the read-only creds bind (a blanket
// chown -R errors on that ro mount).
func TestChownHomeScript(t *testing.T) {
	if s := chownHomeScript("", containerHome); s != "" {
		t.Errorf("root/unset must emit no chown, got %q", s)
	}
	s := chownHomeScript("dev", "/home/dev")
	if !strings.Contains(s, "chown dev:dev") {
		t.Errorf("chown must target the user: %q", s)
	}
	if !strings.Contains(s, "-path /home/dev/.claude/.credentials.json -prune") {
		t.Errorf("chown must prune the ro creds bind (finding 3): %q", s)
	}
	if strings.Contains(s, "chown -R") {
		t.Errorf("must NOT use chown -R (walks the ro creds bind): %q", s)
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

// gitProject upgrades a tempProject into a real git repo with a .githooks dir,
// using a hermetic env (GIT_* stripped, config scopes pinned to /dev/null —
// LL-006/LL-010: fixtures must never act on the real repo).
func gitProject(t *testing.T) string {
	t.Helper()
	root := tempProject(t)
	env := []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	for _, args := range [][]string{{"init", "-b", "main"}} {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func localHooksPath(t *testing.T, root string) string {
	t.Helper()
	c := exec.Command("git", "-C", root, "config", "--local", "--get", "core.hooksPath")
	c.Env = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "PATH=" + os.Getenv("PATH")}
	out, _ := c.Output()
	return strings.TrimSpace(string(out))
}

// TestBuildWiresGithooks (C1): a project shipping .githooks gets
// core.hooksPath converged by build — commit-time guards run by mechanism,
// not by remembering `make hooks`. Idempotent: a wired repo re-audits nothing.
func TestBuildWiresGithooks(t *testing.T) {
	root := gitProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}}

	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := localHooksPath(t, root); got != ".githooks" {
		t.Fatalf("core.hooksPath = %q, want .githooks", got)
	}

	// Second build: already wired → no further githooks.wire write.
	wired, err := ensureGithooksPath(root)
	if err != nil {
		t.Fatalf("ensureGithooksPath: %v", err)
	}
	if wired {
		t.Error("already-wired repo should be a no-op")
	}
}

// TestBuildSkipsGithooksWhenAbsent: no .githooks dir or not a git repo →
// silent skip, never an error (most projects in the wild).
func TestBuildSkipsGithooksWhenAbsent(t *testing.T) {
	root := tempProject(t) // not a git repo
	wired, err := ensureGithooksPath(root)
	if err != nil || wired {
		t.Errorf("non-repo should skip: wired=%t err=%v", wired, err)
	}
}

// TestDoctorGradesGithooksWiring (C1): doctor reports the git-side guardrail
// wiring — .githooks present but core.hooksPath unset is a red check, and a
// project without .githooks gets no verdict at all.
func TestDoctorGradesGithooksWiring(t *testing.T) {
	root := gitProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{exists: true, running: true}

	res, err := doctorImpl(DoctorOpts{PlaybookPath: pbPath}, rt)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if c, ok := checksByName(res)["host:githooks"]; !ok || c.OK {
		t.Errorf("unwired .githooks should fail, got %+v", c)
	}

	if _, err := ensureGithooksPath(root); err != nil {
		t.Fatal(err)
	}
	res, _ = doctorImpl(DoctorOpts{PlaybookPath: pbPath}, rt)
	if c, ok := checksByName(res)["host:githooks"]; !ok || !c.OK {
		t.Errorf("wired .githooks should pass, got %+v", c)
	}

	plain := tempProject(t)
	res, _ = doctorImpl(DoctorOpts{PlaybookPath: filepath.Join(plain, "loom.yml")}, rt)
	if _, ok := checksByName(res)["host:githooks"]; ok {
		t.Error("project without .githooks must get no githooks verdict")
	}
}

// hermeticGit runs git in root with config scopes pinned away from the real
// machine (LL-006/LL-010) and a fixed identity so commits/worktrees succeed.
func hermeticGit(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	c.Env = []string{
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	}
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// TestEnsureGithooksSkipsWorktree (D1 confer / LL-016 class): building from a
// linked worktree (`loom build --playbook <worktree>/loom.yml`) must NOT wire
// core.hooksPath. A worktree's `git config --local` writes the SHARED common
// config, so wiring there clobbers the main checkout's hooksPath as a side
// effect of a worktree build — and the gitdir pointer can be unresolvable from
// the build host (git exit 128, which used to abort the whole build before the
// container step). A worktree (`.git` is a pointer FILE, not a dir) => no-op;
// the main checkout's wiring stays intact.
func TestEnsureGithooksSkipsWorktree(t *testing.T) {
	main := t.TempDir()
	hermeticGit(t, main, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(main, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	hermeticGit(t, main, "commit", "-m", "init", "--allow-empty")
	// The common config points at an ABSOLUTE hooks dir — the real loom repo's
	// state (.git/config: core.hooksPath=/workspace/loom/.githooks). A worktree
	// shares this config, and the literal "!= .githooks" guard would otherwise
	// see the absolute value and try to re-set it: that write clobbers the
	// shared config (and risks the exit-128 abort). The fix skips the worktree
	// before any config call, so this absolute value must survive untouched.
	absHooks := filepath.Join(main, ".githooks")
	hermeticGit(t, main, "config", "--local", "core.hooksPath", absHooks)

	wt := filepath.Join(t.TempDir(), "wt")
	hermeticGit(t, main, "worktree", "add", "-q", wt)
	if err := os.MkdirAll(filepath.Join(wt, ".githooks"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Worktree build: a no-op for hooks, and never an error (non-fatal path).
	wired, err := ensureGithooksPath(wt)
	if err != nil {
		t.Fatalf("worktree ensureGithooksPath must not error (build would abort): %v", err)
	}
	if wired {
		t.Error("worktree build must NOT wire core.hooksPath — it clobbers the shared common config")
	}
	// The main checkout's absolute wiring is untouched by the worktree build.
	if got := localHooksPath(t, main); got != absHooks {
		t.Errorf("main core.hooksPath = %q after worktree build, want %q (shared config clobbered)", got, absHooks)
	}
}

// TestBuildSummaryCountsWrites (live-build e2e F-a): the human summary must
// distinguish "ensured" (full declared set, idempotence) from "written"
// (what THIS run changed) — it once printed "4 materialized" on a 0-write
// re-run, indistinguishable from a run that rewrote the world.
func TestBuildSummaryCountsWrites(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}}

	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	if res.Written == 0 || res.Written != len(res.Materialized) {
		t.Errorf("first build: written=%d materialized=%d, want all written", res.Written, len(res.Materialized))
	}

	res2, err := buildImpl(BuildOpts{PlaybookPath: pbPath},
		fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "exists"}}, fixedClock)
	if err != nil {
		t.Fatalf("re-build: %v", err)
	}
	if res2.Written != 0 || len(res2.Materialized) == 0 {
		t.Errorf("idempotent re-build: written=%d materialized=%d, want 0 written / full ensured set",
			res2.Written, len(res2.Materialized))
	}
	if !strings.Contains(res2.Human(), "0 written") {
		t.Errorf("human summary must carry the write count, got %q", res2.Human())
	}
}

// TestBuildAuditsHomeSync (live-build e2e F-b): the bulk docker cp ships every
// staged file — files no staging-write entry names. When the runtime reports
// the sync happened, the audit log carries ONE home.sync entry naming the
// shipped set; when it didn't (exists/no-op), no such entry is invented.
func TestBuildAuditsHomeSync(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")

	// Converge WITH a home sync but zero staging writes (the e2e shape:
	// "4 materialized, only 2 audited" — now the sync itself is the entry).
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath},
		fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}}, fixedClock); err != nil {
		t.Fatalf("seed build: %v", err)
	}
	res, err := buildImpl(BuildOpts{PlaybookPath: pbPath},
		fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "converged", HomeSynced: true}}, fixedClock)
	if err != nil {
		t.Fatalf("converge build: %v", err)
	}
	if res.Written != 0 {
		t.Fatalf("precondition: want a 0-staging-write run, got written=%d", res.Written)
	}
	data, err := os.ReadFile(filepath.Join(root, ".loom", "actions.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"home.sync"`) {
		t.Error("audit log missing the home.sync entry for a synced run")
	}
	if !strings.Contains(string(data), "prompt.go.sh") {
		t.Error("home.sync entry should name the shipped files")
	}
	// A no-op run (exists, no sync) must not invent one.
	before := countLogLines(t, root)
	if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath},
		fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "exists"}}, fixedClock); err != nil {
		t.Fatalf("noop build: %v", err)
	}
	if got := countLogLines(t, root); got != before {
		t.Errorf("no-op run appended %d audit entries, want 0", got-before)
	}
}

// TestDiagLogAppendsAcrossRuns (live-build e2e F-c): the per-verb diagnostic
// log must accumulate runs (separator-demarcated), never truncate — run 1's
// forensics were gone by run 2.
func TestDiagLogAppendsAcrossRuns(t *testing.T) {
	root := tempProject(t)
	pbPath := filepath.Join(root, "loom.yml")
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}}
	for i := 0; i < 2; i++ {
		if _, err := buildImpl(BuildOpts{PlaybookPath: pbPath}, rt, fixedClock); err != nil {
			t.Fatalf("build %d: %v", i+1, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".loom", "logs", "build.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "----- loom build run -----"); got != 2 {
		t.Errorf("diag log holds %d run separators, want 2 (append, not truncate)\n---\n%s", got, data)
	}
}
