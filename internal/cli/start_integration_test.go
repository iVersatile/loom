//go:build integration

// Integration tier (RULES §7): real docker-backed e2e for the guided run.
// Compiled only under `-tags integration` (make gate-integration / CI); skips
// cleanly when no daemon is present. Runs on the #75 OOM-flaking tier, so the
// fixture is kept MINIMAL — it reuses the same testdata project the build e2e
// (FR-BUILD-008) uses, adding no heavier toolchain.
package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker binary not available")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not available")
	}
}

// TestStartOneRunWorkingEnv (FR-RUN-004) is the frozen one-run success contract:
// from a clean (verified-absent) fixture, ONE `loom start --non-interactive`
// reaches a working env — build converged AND a smoke exec in the env succeeds —
// and start reports ready:true. No restarts, no second invocation.
func TestStartOneRunWorkingEnv(t *testing.T) {
	requireDocker(t)

	dir := tempCopy(t, "../playbook/testdata/proj")
	pb := filepath.Join(dir, "loom.yml")
	const cname = "loom-dev" // project "loom" + "-dev"

	// Clean-machine proxy precondition: verified absent, not assumed.
	_ = exec.Command("docker", "rm", "-f", cname).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", cname).Run() })
	if _, err := os.Stat(filepath.Join(dir, "loom.lock")); !os.IsNotExist(err) {
		t.Fatalf("precondition: loom.lock should be absent before start (err=%v)", err)
	}

	out, _, err := runCmd(t,
		"start", "--non-interactive", "--json",
		"--stack", "go", "--key", "ANTHROPIC_API_KEY=x", // gitleaks:allow — dummy pass-through (value "x"); the smoke exec needs no real key
		"-f", pb)
	if err != nil {
		if data, e := os.ReadFile(filepath.Join(dir, ".loom", "logs", "build.log")); e == nil {
			t.Logf("=== build.log ===\n%s", data)
		}
		t.Fatalf("one-run start failed (want exit 0/ready): %v\n%s", err, out)
	}

	var res startResult
	if e := json.Unmarshal([]byte(out), &res); e != nil {
		t.Fatalf("start --json is not the documented shape: %v\n%s", e, out)
	}
	if !res.Ready {
		t.Fatalf("ready = false after one start run; want true. steps=%+v", res.Steps)
	}

	// The container is real and enterable: an independent smoke exec succeeds —
	// the same property start gated ready:true on.
	if e := exec.Command("docker", "exec", cname, "true").Run(); e != nil {
		t.Errorf("independent smoke exec into %s failed: %v", cname, e)
	}
}
