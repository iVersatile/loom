//go:build integration

// Integration tier (RULES §7): real docker-backed e2e. Compiled only under
// `-tags integration` (make gate-integration / CI); skips cleanly when no daemon
// is present. Proves Phase 1 exit criteria that need a real container.
package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/lock"
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

// TestE2EBuildAndSurviveRebuild covers the spine end-to-end and the survive-
// rebuild property: build → container exists with $HOME config → teardown the
// container → rebuild → $HOME config is back (reconciled from the config source).
func TestE2EBuildAndSurviveRebuild(t *testing.T) {
	requireDocker(t)
	root := tempProject(t)
	pb := filepath.Join(root, "loom.yml")
	name := containerName("loom")
	// Remove any leftover from a prior interrupted run so the build is fresh, and
	// clean up afterward.
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	if _, err := Detect(DetectOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if res, err := Build(BuildOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("build: %v (result=%+v)", err, res)
	}

	assertInContainer := func(path string) {
		if err := exec.Command("docker", "exec", name, "test", "-f", path).Run(); err != nil {
			t.Errorf("expected %s inside container: %v", path, err)
		}
	}
	if ok, _ := (dockerRuntime{}).Exists(name); !ok {
		t.Fatal("container should exist after build")
	}
	assertInContainer("/root/.claude/settings.json")
	assertInContainer("/root/.claude/statusline.sh")

	// The lock pins the base image by manifest digest.
	if l, err := lock.Read(filepath.Join(root, "loom.lock")); err != nil || l == nil {
		t.Errorf("read lock: %v", err)
	} else if !strings.Contains(l.BaseImage, "@sha256:") {
		t.Errorf("lock base_image %q should be digest-pinned", l.BaseImage)
	}

	// Provisioned toolchain is usable inside the container.
	if out, err := exec.Command("docker", "exec", name, "sh", "-lc",
		"PATH=$PATH:/usr/local/go/bin go version").CombinedOutput(); err != nil {
		t.Errorf("go not provisioned in container: %v: %s", err, out)
		// Dump the diagnostic log so the provisioning trace is visible (the temp
		// project dir is removed after the test).
		if data, e := os.ReadFile(filepath.Join(root, ".loom", "logs", "build.log")); e == nil {
			t.Logf("=== build.log ===\n%s", data)
		}
	}
	assertInContainer("/root/go/bin/gopls")

	// Tear the container down, rebuild, and confirm $HOME config returns.
	if _, err := Teardown(TeardownOpts{PlaybookPath: pb, Level: "stop"}); err != nil {
		t.Fatalf("teardown: %v", err)
	}
	if _, err := Build(BuildOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertInContainer("/root/.claude/settings.json")
}
