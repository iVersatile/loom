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
//
// It is also the clean-machine proxy (FR-BUILD-008, Phase-1 exit criterion 1
// per the T1 doctrine): the start state is VERIFIED absent — no container, no
// lockfile — so "one unattended build converges to a working env" is proven
// literally, from nothing. The guided-run experience itself remains human
// feedback, never this FR's coverage.
func TestE2EBuildAndSurviveRebuild(t *testing.T) {
	requireDocker(t)
	root := tempProject(t)
	pb := filepath.Join(root, "loom.yml")
	name := containerName("loom")
	// Remove any leftover from a prior interrupted run so the build is fresh, and
	// clean up afterward.
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	// Clean-machine proxy precondition: verified absent, not assumed absent.
	if ok, _ := (dockerRuntime{}).Exists(name); ok {
		t.Fatal("precondition: container should be absent before the clean build")
	}
	if _, err := os.Stat(filepath.Join(root, "loom.lock")); !os.IsNotExist(err) {
		t.Fatalf("precondition: loom.lock should be absent before the clean build (err=%v)", err)
	}

	if _, err := Detect(DetectOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if res, err := Build(BuildOpts{PlaybookPath: pb}); err != nil {
		// Dump the diagnostic log so the provisioning trace is visible (the temp
		// project dir is removed after the test).
		if data, e := os.ReadFile(filepath.Join(root, ".loom", "logs", "build.log")); e == nil {
			t.Logf("=== build.log ===\n%s", data)
		}
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

// TestE2EDotfileChangeConverges is the T7 regression test: a dotfile-only
// change on an EXISTING container must reach the container on the next build
// (no --force, no teardown), and the result must reflect the convergence.
// Before the fix, the home re-sync was gated on the toolset digest alone, so
// the container silently kept the old file.
func TestE2EDotfileChangeConverges(t *testing.T) {
	requireDocker(t)
	root := tempProject(t)
	pb := filepath.Join(root, "loom.yml")
	name := containerName("loom")
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	if _, err := Build(BuildOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Edit a dotfile in the config source (the T7 trigger: dotfile-only change).
	marker := "echo t7-resync-marker"
	src := filepath.Join(root, "config", "dotfiles", "claude", "statusline.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"+marker+"\n"), 0o755); err != nil {
		t.Fatalf("edit dotfile: %v", err)
	}

	res, err := Build(BuildOpts{PlaybookPath: pb})
	if err != nil {
		t.Fatalf("rebuild after dotfile edit: %v", err)
	}
	if res.Container.Status != "converged" {
		t.Errorf("container status = %q, want converged (home drift must reconcile)", res.Container.Status)
	}
	out, err := exec.Command("docker", "exec", name, "cat", "/root/.claude/statusline.sh").CombinedOutput()
	if err != nil {
		t.Fatalf("read statusline in container: %v: %s", err, out)
	}
	if !strings.Contains(string(out), marker) {
		t.Errorf("container statusline not re-synced: got %q, want it to contain %q", out, marker)
	}

	// Idempotent: a third build with nothing changed must not re-converge.
	res, err = Build(BuildOpts{PlaybookPath: pb})
	if err != nil {
		t.Fatalf("idempotent rebuild: %v", err)
	}
	if res.Container.Status != "exists" {
		t.Errorf("container status = %q, want exists (nothing left to sync)", res.Container.Status)
	}
}

// TestE2EExecContract proves the SPEC-verbs exec contract against a real
// container (FR-EXEC-001/002): login-shell env (provisioned PATH finds go),
// workspace cwd, and verbatim non-zero exit propagation. The dogfood form of
// this test is `loom exec -- make gate` inside loom-dev (T12 criterion 5,
// loom-native) — the fixture container has no make, so the contract is proven
// with equivalents.
func TestE2EExecContract(t *testing.T) {
	requireDocker(t)
	root := tempProject(t)
	pb := filepath.Join(root, "loom.yml")
	name := containerName("loom")
	_ = exec.Command("docker", "rm", "-f", name).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	if _, err := Build(BuildOpts{PlaybookPath: pb}); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Login env: the provisioned PATH (.profile: /usr/local/go/bin) must apply.
	if res, err := Exec(ExecOpts{PlaybookPath: pb, Command: []string{"go", "version"}}); err != nil || res.ExitCode != 0 {
		t.Errorf("exec go version: exit=%d err=%v — login-shell PATH not applied?", res.ExitCode, err)
	}
	// Workspace cwd: the command runs in the project mount.
	if res, err := Exec(ExecOpts{PlaybookPath: pb, Command: []string{"sh", "-c", `test "$PWD" = /workspace/loom`}}); err != nil || res.ExitCode != 0 {
		t.Errorf("exec cwd check: exit=%d err=%v — want /workspace/loom", res.ExitCode, err)
	}
	// Verbatim exit propagation, non-zero, with no engine error.
	if res, err := Exec(ExecOpts{PlaybookPath: pb, Command: []string{"sh", "-c", "exit 7"}}); err != nil || res.ExitCode != 7 {
		t.Errorf("exec exit 7: exit=%d err=%v — want verbatim 7, no error", res.ExitCode, err)
	}

	// Stopped → started, then entered (idempotent bring-up).
	if out, err := exec.Command("docker", "stop", name).CombinedOutput(); err != nil {
		t.Fatalf("docker stop: %v: %s", err, out)
	}
	if res, err := Exec(ExecOpts{PlaybookPath: pb, Command: []string{"true"}}); err != nil || res.ExitCode != 0 {
		t.Errorf("exec after stop: exit=%d err=%v — want start-then-enter", res.ExitCode, err)
	}
}
