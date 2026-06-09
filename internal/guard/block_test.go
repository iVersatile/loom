package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the actual guardrail SHELL scripts (mechanism, ADR-0005),
// proving Phase 1 exit criterion "guardrails block a destructive test" — and the
// audited override path — without needing docker. They run in the normal gate.

func absHook(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "config", "hooks", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runHook(dir string, env []string, args ...string) error {
	c := exec.Command("sh", args...)
	c.Dir = dir
	// Hermetic: the audited ALLOW_* overrides must come ONLY from the test's
	// explicit env, never leak in from the ambient shell. Otherwise running the
	// suite under `ALLOW_SPEC_CHANGE=1 git commit ...` would make a "should BLOCK"
	// assertion silently pass — the spec guard's own test defeated by the override
	// it polices. Strip them from the inherited env first (LL-006).
	c.Env = append(withoutOverrides(os.Environ()), env...)
	return c.Run()
}

// withoutOverrides drops audited ALLOW_* override vars from an environment so a
// hook test only sees the overrides a case passes explicitly.
func withoutOverrides(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "ALLOW_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func newGitRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-b", branch)
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	// An initial commit so HEAD is born and rev-parse reports the branch (as it
	// would in any real repo a pre-commit hook runs in).
	git("commit", "--allow-empty", "-m", "init")
	return dir
}

func TestGuardBashBlocksDangerousAllowsBenign(t *testing.T) {
	hook := absHook(t, "guard-bash")
	// Assembled at runtime so neither the test source nor the harness deny-list
	// is the thing under test — the script is.
	danger := strings.Join([]string{"rm", "-rf", "/"}, " ")
	if runHook("", nil, hook, danger) == nil {
		t.Error("guard-bash should BLOCK a destructive command")
	}
	if err := runHook("", nil, hook, "git status"); err != nil {
		t.Errorf("guard-bash should allow a benign command: %v", err)
	}
}

func TestBranchGuardBlocksMainAllowsOverrideAndBranch(t *testing.T) {
	hook := absHook(t, "branch-guard")

	main := newGitRepo(t, "main")
	if runHook(main, nil, hook) == nil {
		t.Error("branch-guard should BLOCK on main")
	}
	if err := runHook(main, []string{"ALLOW_MAIN_COMMIT=1"}, hook); err != nil {
		t.Errorf("ALLOW_MAIN_COMMIT=1 should permit (audited): %v", err)
	}

	feat := newGitRepo(t, "feat/x")
	if err := runHook(feat, nil, hook); err != nil {
		t.Errorf("branch-guard should allow a feature branch: %v", err)
	}
}

func TestProtectPathsBlocksFrozenContract(t *testing.T) {
	hook := absHook(t, "protect-paths")
	repo := newGitRepo(t, "feat/x")

	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "SPEC-x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", "docs/SPEC-x.md")
	add.Dir = repo
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}

	if runHook(repo, nil, hook) == nil {
		t.Error("protect-paths should BLOCK a staged frozen-contract change")
	}
	if err := runHook(repo, []string{"ALLOW_SPEC_CHANGE=1"}, hook); err != nil {
		t.Errorf("ALLOW_SPEC_CHANGE=1 should permit (audited): %v", err)
	}
}
