package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// These tests pin scripts/cold-floor-cron — the ADR-0022 cold-start floor (option B,
// 7-day trial) host-cron ACTUATOR GLUE. The decision itself (NUDGE / NO-OP /
// HALT-blocked) is pinned by TestColdCheck; these pin the DISPATCH the glue adds:
// a NUDGE verdict runs $LOOM_COLD_NUDGE_CMD, every other verdict (NO-OP /
// HALT-blocked) injects nothing, an empty inject command is a safe dry-run, and a
// missing LOOM_REPO fails fast (never a silent no-op that would hide a misconfig).

func coldFloorCronScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "cold-floor-cron"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// fakeCheck writes a stub verdict source that just echoes a fixed verdict, so the
// dispatch is exercised hermetically without depending on inbox/drain state.
func fakeCheck(t *testing.T, verdict string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-check")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nprintf '%s\\n' "+shellQuote(verdict)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func shellQuote(s string) string {
	return "'" + s + "'" // verdict strings here contain no single quotes
}

// runCron runs cold-floor-cron against a fake verdict and an inject command that
// touches a sentinel; it reports whether the inject ran.
func runCron(t *testing.T, verdict string, withInject bool) (injected bool) {
	t.Helper()
	repo := t.TempDir()
	sentinel := filepath.Join(repo, "injected")
	cmd := ""
	if withInject {
		cmd = "touch " + sentinel
	}
	c := exec.Command("sh", coldFloorCronScript(t))
	c.Env = append(hermeticEnv(),
		"LOOM_REPO="+repo,
		"LOOM_COLD_CHECK="+fakeCheck(t, verdict),
		"LOOM_COLD_NUDGE_CMD="+cmd,
	)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("cold-floor-cron: %v\n%s", err, out)
	}
	_, statErr := os.Stat(sentinel)
	return statErr == nil
}

func TestColdFloorCron(t *testing.T) {
	t.Run("NUDGE ⇒ inject runs", func(t *testing.T) {
		if !runCron(t, "NUDGE (1 deliverable queued; host may wake the idle Writer)", true) {
			t.Fatal("a NUDGE verdict must run LOOM_COLD_NUDGE_CMD")
		}
	})

	t.Run("NO-OP ⇒ inject does NOT run", func(t *testing.T) {
		if runCron(t, "NO-OP (no deliverable queued work — never wake on nothing)", true) {
			t.Fatal("a NO-OP verdict must never inject")
		}
	})

	t.Run("HALT-blocked ⇒ inject does NOT run", func(t *testing.T) {
		if runCron(t, "HALT-blocked (HALT sentinel present — nudge nothing)", true) {
			t.Fatal("a HALTed seat must never be injected")
		}
	})

	t.Run("empty inject cmd ⇒ NUDGE is a safe dry-run", func(t *testing.T) {
		if runCron(t, "NUDGE (1 deliverable queued)", false) {
			t.Fatal("an empty LOOM_COLD_NUDGE_CMD must inject nothing and must not error")
		}
	})

	t.Run("missing LOOM_REPO ⇒ fail-fast", func(t *testing.T) {
		c := exec.Command("sh", coldFloorCronScript(t))
		c.Env = hermeticEnv() // no LOOM_REPO
		if err := c.Run(); err == nil {
			t.Fatal("a missing LOOM_REPO must fail fast, not silently no-op")
		}
	})
}
