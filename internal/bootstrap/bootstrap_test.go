// Package bootstrap holds the conformance tests for the entry: bootstrap
// clause (SPEC-verbs, accepted #64) — FR-ENTRY-001..004. There is no Go code
// to test: the subject is bootstrap/loom-bootstrap.sh, exec'd hermetically
// (LL-008: no dependence on host-installed tooling; the PATH each test sees
// is constructed, so "go absent" is a fact, not an accident of the host).
package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fixture builds a fake repo root containing only the real bootstrap script,
// plus a minimal PATH dir holding the external tools the script needs
// (dirname; everything else is a shell builtin). `go` is NOT on this PATH.
func fixture(t *testing.T) (root, binPath string) {
	t.Helper()
	root = t.TempDir()
	for _, d := range []string{"bootstrap", "bin", "cmd", "path"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "bootstrap", "loom-bootstrap.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "bootstrap", "loom-bootstrap.sh")
	if err := os.WriteFile(script, src, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath = filepath.Join(root, "path")
	for _, tool := range []string{"dirname"} {
		real, err := exec.LookPath(tool)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, filepath.Join(binPath, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return root, binPath
}

// runBootstrap executes the script with a constructed PATH and clean env.
func runBootstrap(t *testing.T, root, binPath string, extraEnv []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	script := filepath.Join(root, "bootstrap", "loom-bootstrap.sh")
	c := exec.Command("sh", append([]string{script}, args...)...)
	c.Dir = root
	c.Env = append([]string{"PATH=" + binPath, "HOME=" + root}, extraEnv...)
	var out, errb strings.Builder
	c.Stdout, c.Stderr = &out, &errb
	err := c.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("bootstrap did not run: %v (stderr: %s)", err, errb.String())
	}
	return out.String(), errb.String(), code
}

func writeFakeLoom(t *testing.T, path, marker string, exitCode int) {
	t.Helper()
	body := fmt.Sprintf("#!/bin/sh\necho \"%s $@\"\nexit %d\n", marker, exitCode)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// FR-ENTRY-001: an existing executable at LOOM_BIN (default bin/loom) is
// exec'd as-is — no rebuild, no mutation. Proven with go ABSENT from PATH:
// any rebuild attempt would fail loudly.
func TestBootstrapExecsExistingBinary(t *testing.T) {
	root, binPath := fixture(t)
	writeFakeLoom(t, filepath.Join(root, "bin", "loom"), "FAKE_LOOM", 7)

	stdout, stderr, code := runBootstrap(t, root, binPath, nil, "doctor", "--json")
	if !strings.Contains(stdout, "FAKE_LOOM doctor --json") {
		t.Fatalf("existing binary not exec'd with args; stdout=%q stderr=%q", stdout, stderr)
	}
	if code != 7 {
		t.Fatalf("exit code not propagated from the exec'd binary: got %d, want 7", code)
	}
	if strings.Contains(stderr, "building engine") {
		t.Fatalf("rebuild attempted despite existing binary: %q", stderr)
	}
}

// FR-ENTRY-002: no binary + Go toolchain present => builds ./cmd/loom to
// bin/loom, then execs it. A fake `go` shim proves the invocation shape
// without compiling anything.
func TestBootstrapBuildsViaGoWhenAbsent(t *testing.T) {
	root, binPath := fixture(t)
	goLog := filepath.Join(root, "go.log")
	shim := `#!/bin/sh
echo "GO $@" >> "` + goLog + `"
out=""; prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out="$a"; prev="$a"; done
printf '#!/bin/sh\necho BUILT_LOOM "$@"\nexit 0\n' > "$out"
chmod +x "$out"
`
	if err := os.WriteFile(filepath.Join(binPath, "go"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	// chmod + printf live outside the minimal PATH inside the shim — give the
	// shim a real shell environment for its own body only.
	for _, tool := range []string{"chmod", "printf"} {
		if real, err := exec.LookPath(tool); err == nil {
			_ = os.Symlink(real, filepath.Join(binPath, tool))
		}
	}

	stdout, stderr, code := runBootstrap(t, root, binPath, nil, "plan")
	logb, _ := os.ReadFile(goLog)
	if !regexp.MustCompile(`GO build -o \S*/bin/loom \./cmd/loom`).Match(logb) {
		t.Fatalf("go not invoked as 'build -o <root>/bin/loom ./cmd/loom'; log=%q stderr=%q", logb, stderr)
	}
	if !strings.Contains(stdout, "BUILT_LOOM plan") {
		t.Fatalf("freshly built binary not exec'd with args; stdout=%q", stdout)
	}
	if code != 0 {
		t.Fatalf("exit code: got %d, want 0", code)
	}
}

// FR-ENTRY-003: no binary + no Go => exactly ONE stderr line naming BOTH
// remedies (install Go >=1.26 | prebuilt at bin/loom or LOOM_BIN), exit 1.
func TestBootstrapFailsNamingBothRemedies(t *testing.T) {
	root, binPath := fixture(t)

	stdout, stderr, code := runBootstrap(t, root, binPath, nil, "build")
	if code != 1 {
		t.Fatalf("exit code: got %d, want 1 (stdout=%q)", code, stdout)
	}
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("failure contract is EXACTLY one stderr line; got %d: %q", len(lines), stderr)
	}
	line := lines[0]
	for _, want := range []string{"Go >=1.26", "bin/loom", "LOOM_BIN"} {
		if !strings.Contains(line, want) {
			t.Fatalf("failure line missing remedy %q: %q", want, line)
		}
	}
}

// FR-ENTRY-004 (automatable proxy per ADR-0013): bootstrap performs no
// network access. Static scan — the script invokes no fetch tooling and
// embeds no URL; the dynamic complement is FR-ENTRY-002's shim log showing
// `go build` as the only external invocation on the build path.
func TestBootstrapInvokesNoNetworkTooling(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "bootstrap", "loom-bootstrap.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`curl|wget|fetch|git[[:space:]]+clone|https?://|nc[[:space:]]`).Find(src); m != nil {
		t.Fatalf("network tooling/URL found in bootstrap script: %q (frozen network-free until the T20-gated amendment)", m)
	}
}
