package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the ADR-0022 decision-trio tools (adv-073/076/077) — the three
// MECHANICAL halves of the /spike + /zoom-out + /confer skills:
//   scripts/ask-other-seat  — queue a framed envelope into the OTHER seat's inbox
//   scripts/cost-vs-gain    — ranked options × {cost,gain,risk,reversibility} table
//   scripts/spike-sandbox   — throwaway probe sandbox with guaranteed teardown
// (exhaust-options is a SKILL-PROSE checklist, not a tool — adv-077 ruling — so it
// has no test here.) Each tool is a discipline turned into mechanism; these tests
// are the guardrail that keeps the mechanism honest.

func trioScript(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// runTrio runs a tool via `sh` and returns combined output + exit code (0 on clean
// exit). Unlike the cold-check helper it never fatals on a non-zero exit — several
// trio behaviours (rejecting bad input, propagating a probe's exit code) ARE the
// non-zero paths under test.
func runTrio(t *testing.T, stdin string, env []string, name string, args ...string) (string, int) {
	t.Helper()
	c := exec.Command("sh", append([]string{trioScript(t, name)}, args...)...)
	c.Env = append(hermeticEnv(), env...)
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	out, err := c.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%s: unexpected error: %v\n%s", name, err, out)
		}
	}
	return string(out), code
}

// ---- ask-other-seat ------------------------------------------------------------

func TestAskOtherSeat(t *testing.T) {
	// inboxDir seeds an empty inbox for both seats under a temp dir.
	inboxDir := func(t *testing.T) string {
		d := t.TempDir()
		for _, r := range []string{"loom-author", "loom-advisor"} {
			if err := os.WriteFile(filepath.Join(d, r+".md"), []byte("AUTOPILOT: on\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return d
	}

	t.Run("author asks advisor ⇒ envelope appended to advisor inbox, QUEUED", func(t *testing.T) {
		d := inboxDir(t)
		env := []string{"LOOM_SESSION_ROLE=loom-author", "LOOM_INBOX_DIR=" + d, "LOOM_ASK_ID=q-test1"}
		out, code := runTrio(t, "", env, "ask-other-seat", "self-wake actuator A vs B", "Which shape do you prefer?")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d\n%s", code, out)
		}
		got, _ := os.ReadFile(filepath.Join(d, "loom-advisor.md"))
		s := string(got)
		for _, want := range []string{"--- id: q-test1", "from: loom-author", "serves: self-wake actuator A vs B", "status: QUEUED", "Which shape do you prefer?"} {
			if !strings.Contains(s, want) {
				t.Errorf("advisor inbox missing %q\n%s", want, s)
			}
		}
		// The asker's OWN inbox must be untouched (it writes only across the seam).
		mine, _ := os.ReadFile(filepath.Join(d, "loom-author.md"))
		if strings.Contains(string(mine), "q-test1") {
			t.Errorf("ask-other-seat wrote into its OWN inbox:\n%s", mine)
		}
	})

	t.Run("advisor asks author ⇒ lands in author inbox (direction flips with role)", func(t *testing.T) {
		d := inboxDir(t)
		env := []string{"LOOM_SESSION_ROLE=loom-advisor", "LOOM_INBOX_DIR=" + d, "LOOM_ASK_ID=q-test2"}
		if _, code := runTrio(t, "", env, "ask-other-seat", "queue shape", "body"); code != 0 {
			t.Fatalf("exit %d", code)
		}
		got, _ := os.ReadFile(filepath.Join(d, "loom-author.md"))
		if !strings.Contains(string(got), "from: loom-advisor") || !strings.Contains(string(got), "q-test2") {
			t.Errorf("author inbox missing advisor's question:\n%s", got)
		}
	})

	t.Run("body from stdin when no body args", func(t *testing.T) {
		d := inboxDir(t)
		env := []string{"LOOM_SESSION_ROLE=loom-author", "LOOM_INBOX_DIR=" + d, "LOOM_ASK_ID=q-stdin"}
		if _, code := runTrio(t, "piped-question-body\n", env, "ask-other-seat", "subject only"); code != 0 {
			t.Fatalf("exit %d", code)
		}
		got, _ := os.ReadFile(filepath.Join(d, "loom-advisor.md"))
		if !strings.Contains(string(got), "piped-question-body") {
			t.Errorf("stdin body not used:\n%s", got)
		}
	})

	t.Run("unknown self role is refused (no envelope, exit 2)", func(t *testing.T) {
		d := inboxDir(t)
		env := []string{"LOOM_SESSION_ROLE=stranger", "LOOM_INBOX_DIR=" + d}
		out, code := runTrio(t, "", env, "ask-other-seat", "x", "y")
		if code == 0 {
			t.Fatalf("an unknown role must not queue anything: %s", out)
		}
	})

	t.Run("injection: a metachar id is sanitized, cannot forge structure", func(t *testing.T) {
		d := inboxDir(t)
		env := []string{"LOOM_SESSION_ROLE=loom-author", "LOOM_INBOX_DIR=" + d, "LOOM_ASK_ID=q $(touch /tmp/loom-ask-pwned) X"}
		if _, code := runTrio(t, "", env, "ask-other-seat", "s", "b"); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if _, err := os.Stat("/tmp/loom-ask-pwned"); err == nil {
			_ = os.Remove("/tmp/loom-ask-pwned")
			t.Fatal("injection: ask-other-seat executed an id-derived command")
		}
		got, _ := os.ReadFile(filepath.Join(d, "loom-advisor.md"))
		// Sanitized id keeps only [A-Za-z0-9-]: spaces and "$(...)" chars dropped.
		if !strings.Contains(string(got), "--- id: qtouchtmploom-ask-pwnedX") {
			t.Errorf("id not sanitized as expected:\n%s", got)
		}
	})
}

// ---- cost-vs-gain --------------------------------------------------------------

func TestCostVsGain(t *testing.T) {
	t.Run("ranks by gain+rev-cost-risk; top row is RECOMMEND", func(t *testing.T) {
		in := "" +
			"Cheap-big-win: cost=1 gain=5 risk=1 reversibility=5\n" +
			"Mid: cost=3 gain=3 risk=3 reversibility=3\n" +
			"Costly-risky: cost=5 gain=2 risk=5 rev=1\n"
		out, code := runTrio(t, in, nil, "cost-vs-gain")
		if code != 0 {
			t.Fatalf("exit %d\n%s", code, out)
		}
		lines := dataLines(out)
		if len(lines) != 3 {
			t.Fatalf("expected 3 ranked rows, got %d:\n%s", len(lines), out)
		}
		if !strings.Contains(lines[0], "Cheap-big-win") || !strings.Contains(lines[0], "RECOMMEND") || !strings.Contains(lines[0], "+8") {
			t.Errorf("rank 1 wrong: %q", lines[0])
		}
		if !strings.Contains(lines[2], "Costly-risky") || !strings.Contains(lines[2], "-7") {
			t.Errorf("rank 3 wrong: %q", lines[2])
		}
	})

	t.Run("stable tie-break keeps input order", func(t *testing.T) {
		in := "" +
			"First: cost=2 gain=2 risk=2 reversibility=2\n" +
			"Second: cost=2 gain=2 risk=2 reversibility=2\n"
		out, _ := runTrio(t, in, nil, "cost-vs-gain")
		lines := dataLines(out)
		if len(lines) != 2 || !strings.Contains(lines[0], "First") || !strings.Contains(lines[1], "Second") {
			t.Errorf("tie-break not stable:\n%s", out)
		}
	})

	t.Run("blank lines and # comments are ignored", func(t *testing.T) {
		in := "# a header comment\n\nOnly: cost=1 gain=1 risk=1 reversibility=1\n\n"
		out, code := runTrio(t, in, nil, "cost-vs-gain")
		if code != 0 || len(dataLines(out)) != 1 {
			t.Errorf("comments/blanks not ignored (code=%d):\n%s", code, out)
		}
	})

	t.Run("an unscored axis is rejected (no silent default)", func(t *testing.T) {
		// risk + reversibility missing — the very hand-wave the tool prevents.
		out, code := runTrio(t, "Bad: cost=1 gain=2\n", nil, "cost-vs-gain")
		if code != 1 {
			t.Errorf("a half-scored option must be rejected with exit 1, got %d:\n%s", code, out)
		}
	})

	t.Run("out-of-range scores are rejected", func(t *testing.T) {
		_, code := runTrio(t, "X: cost=9 gain=1 risk=1 reversibility=1\n", nil, "cost-vs-gain")
		if code != 1 {
			t.Errorf("a >5 score must be rejected, got exit %d", code)
		}
	})
}

// dataLines returns the rendered data rows of a cost-vs-gain table (drops the two
// header rows and any blank trailing line).
func dataLines(out string) []string {
	var rows []string
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if ln == "" || strings.HasPrefix(ln, "rank") || strings.HasPrefix(ln, "----") {
			continue
		}
		rows = append(rows, ln)
	}
	return rows
}

// ---- spike-sandbox -------------------------------------------------------------

func TestSpikeSandbox(t *testing.T) {
	spikeEnv := func(t *testing.T) (spikeDir string, env []string) {
		d := t.TempDir()
		return d, []string{"LOOM_ROOT=" + d, "LOOM_SPIKE_DIR=" + filepath.Join(d, "spikes")}
	}

	t.Run("run mode: probe runs in the sandbox, sandbox is torn down after", func(t *testing.T) {
		d, env := spikeEnv(t)
		out := filepath.Join(d, "cwd.txt")
		// The probe records its cwd to a file OUTSIDE the sandbox so we can inspect
		// it after teardown removes the sandbox itself.
		_, code := runTrio(t, "", env, "spike-sandbox", "probe", "--", "sh", "-c", "pwd > "+out)
		if code != 0 {
			t.Fatalf("run-mode exit %d", code)
		}
		cwd, _ := os.ReadFile(out)
		sandbox := filepath.Join(d, "spikes", "probe.sandbox")
		if strings.TrimSpace(string(cwd)) != sandbox {
			t.Errorf("probe cwd was not the sandbox: got %q want %q", strings.TrimSpace(string(cwd)), sandbox)
		}
		if _, err := os.Stat(sandbox); !os.IsNotExist(err) {
			t.Errorf("sandbox not torn down: %v", err)
		}
	})

	t.Run("a failing probe propagates its exit code (a spike that fails fails)", func(t *testing.T) {
		_, env := spikeEnv(t)
		_, code := runTrio(t, "", env, "spike-sandbox", "fails", "--", "sh", "-c", "exit 7")
		if code != 7 {
			t.Errorf("expected propagated exit 7, got %d", code)
		}
	})

	t.Run("--keep leaves the sandbox; teardown is opt-out", func(t *testing.T) {
		d, env := spikeEnv(t)
		out, code := runTrio(t, "", env, "spike-sandbox", "--keep", "kept", "--", "true")
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		if !strings.Contains(out, "--keep") {
			t.Errorf("--keep should announce the leftover:\n%s", out)
		}
		sandbox := filepath.Join(d, "spikes", "kept.sandbox")
		if _, err := os.Stat(sandbox); err != nil {
			t.Errorf("--keep sandbox should survive: %v", err)
		}
	})

	t.Run("--clean removes a leftover sandbox", func(t *testing.T) {
		d, env := spikeEnv(t)
		sandbox := filepath.Join(d, "spikes", "leftover.sandbox")
		if err := os.MkdirAll(sandbox, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, code := runTrio(t, "", env, "spike-sandbox", "--clean", "leftover"); code != 0 {
			t.Fatalf("clean exit %d", code)
		}
		if _, err := os.Stat(sandbox); !os.IsNotExist(err) {
			t.Errorf("--clean did not remove the sandbox: %v", err)
		}
	})

	t.Run("no command: prints the path and does NOT auto-tear-down", func(t *testing.T) {
		d, env := spikeEnv(t)
		out, code := runTrio(t, "", env, "spike-sandbox", "interactive")
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		sandbox := filepath.Join(d, "spikes", "interactive.sandbox")
		if !strings.Contains(out, sandbox) {
			t.Errorf("path not printed:\n%s", out)
		}
		if _, err := os.Stat(sandbox); err != nil {
			t.Errorf("no-command sandbox should persist for interactive use: %v", err)
		}
	})

	t.Run("injection: probe argv is not evaluated by the wrapper", func(t *testing.T) {
		_, env := spikeEnv(t)
		// The arg LOOKS like a command substitution but reaches `echo` as a literal
		// argv element — the wrapper runs "$@", never a shell string.
		runTrio(t, "", env, "spike-sandbox", "inj", "--", "echo", "$(touch /tmp/loom-spike-pwned)")
		if _, err := os.Stat("/tmp/loom-spike-pwned"); err == nil {
			_ = os.Remove("/tmp/loom-spike-pwned")
			t.Fatal("injection: spike-sandbox evaluated a probe argv")
		}
	})

	t.Run("--worktree provisions a detached git worktree and removes it on teardown", func(t *testing.T) {
		repo := newGitRepo(t, "main")
		spikes := filepath.Join(t.TempDir(), "spikes")
		env := []string{"LOOM_ROOT=" + repo, "LOOM_SPIKE_DIR=" + spikes}
		out := filepath.Join(t.TempDir(), "isrepo.txt")
		// Inside a detached worktree, `git rev-parse --is-inside-work-tree` is true
		// and HEAD is detached. Record both to a file outside the sandbox.
		_, code := runTrio(t, "", env, "spike-sandbox", "--worktree", "wt", "--",
			"sh", "-c", "git rev-parse --is-inside-work-tree > "+out)
		if code != 0 {
			t.Fatalf("worktree run exit %d", code)
		}
		if got, _ := os.ReadFile(out); strings.TrimSpace(string(got)) != "true" {
			t.Errorf("probe did not run inside a work tree: %q", got)
		}
		sandbox := filepath.Join(spikes, "wt.sandbox")
		if _, err := os.Stat(sandbox); !os.IsNotExist(err) {
			t.Errorf("worktree sandbox not removed: %v", err)
		}
		// And the worktree must be deregistered, not just the dir deleted.
		list, _ := exec.Command("git", "-C", repo, "worktree", "list").CombinedOutput()
		if strings.Contains(string(list), sandbox) {
			t.Errorf("worktree still registered after teardown:\n%s", list)
		}
	})
}
