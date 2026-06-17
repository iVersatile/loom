package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin scripts/cold-check — the ADR-0022 cold-start floor decision
// (adv-078, option-B 7-day trial). It is the offline "should the host nudge the
// idle Writer now?" call: HALT-first, NUDGE iff resurface-decide reports >= 1
// deliverable (QUEUED / dep-cleared PARKED) item, else NO-OP (zero spurious
// nudges). It reuses config/hooks/resurface-decide as the single deliverable
// truth, so this floor cannot drift from the drain.

func coldCheckScript(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "cold-check"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// coldFixture writes an inbox + merged-refs + log under a temp dir and returns
// their paths plus the HALT sentinel path (created on demand by the caller).
func coldFixture(t *testing.T, inbox, merged string) (inboxPath, mergedPath, haltPath, logPath string) {
	t.Helper()
	dir := t.TempDir()
	inboxPath = filepath.Join(dir, "loom-author.md")
	mergedPath = filepath.Join(dir, ".merged-prs")
	haltPath = filepath.Join(dir, "HALT")
	logPath = filepath.Join(dir, ".cold-floor-log")
	if err := os.WriteFile(inboxPath, []byte(inbox), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mergedPath, []byte(merged), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func runColdCheck(t *testing.T, inbox, merged, halt, log string, env ...string) string {
	t.Helper()
	c := exec.Command("sh", coldCheckScript(t), inbox, merged, halt, log)
	c.Env = append(append(hermeticEnv(), "LOOM_NOW=1000"), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("cold-check: %v\n%s", err, out)
	}
	return string(out)
}

func TestColdCheck(t *testing.T) {
	t.Run("QUEUED work ⇒ NUDGE + logged", func(t *testing.T) {
		inbox, merged, halt, log := coldFixture(t,
			"AUTOPILOT: on\n\n--- id: A99\nstatus: QUEUED\n\nbody\n", "")
		out := runColdCheck(t, inbox, merged, halt, log)
		if !strings.HasPrefix(out, "NUDGE") {
			t.Fatalf("queued work must NUDGE, got: %s", out)
		}
		body, _ := os.ReadFile(log)
		if !strings.Contains(string(body), "NUDGE") || !strings.Contains(string(body), "nudged=yes") ||
			!strings.Contains(string(body), "A99") {
			t.Errorf("trial log must record the nudge with the item id:\n%s", body)
		}
	})

	t.Run("dep-cleared PARKED ⇒ NUDGE (deliverable truth tracks the drain)", func(t *testing.T) {
		inbox, merged, halt, log := coldFixture(t,
			"\n--- id: P01\nparked-on: pr-merged:123\nstatus: PARKED\n\nbody\n", "123\n")
		out := runColdCheck(t, inbox, merged, halt, log)
		if !strings.HasPrefix(out, "NUDGE") {
			t.Fatalf("a PARKED item whose dep just merged is deliverable ⇒ NUDGE, got: %s", out)
		}
	})

	t.Run("no deliverable work ⇒ NO-OP (zero spurious nudges)", func(t *testing.T) {
		// DONE item + an uncleared PARKED item: the drain delivers neither.
		inbox, merged, halt, log := coldFixture(t,
			"\n--- id: B01\nstatus: DONE\n\n--- id: P02\nparked-on: pr-merged:999\nstatus: PARKED\n", "")
		out := runColdCheck(t, inbox, merged, halt, log)
		if !strings.HasPrefix(out, "NO-OP") {
			t.Fatalf("no deliverable work must NO-OP, got: %s", out)
		}
		body, _ := os.ReadFile(log)
		if strings.Contains(string(body), "nudged=yes") {
			t.Errorf("a NO-OP must never record a nudge (zero spurious):\n%s", body)
		}
	})

	t.Run("empty inbox ⇒ NO-OP", func(t *testing.T) {
		inbox, merged, halt, log := coldFixture(t, "", "")
		out := runColdCheck(t, inbox, merged, halt, log)
		if !strings.HasPrefix(out, "NO-OP") {
			t.Fatalf("empty inbox must NO-OP, got: %s", out)
		}
	})

	t.Run("HALT beats NUDGE (HALT-first, never nudge a halted seat)", func(t *testing.T) {
		inbox, merged, halt, log := coldFixture(t,
			"\n--- id: C01\nstatus: QUEUED\n", "")
		if err := os.WriteFile(halt, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		out := runColdCheck(t, inbox, merged, halt, log)
		if !strings.HasPrefix(out, "HALT-blocked") {
			t.Fatalf("HALT must short-circuit before the inbox read even with queued work, got: %s", out)
		}
		body, _ := os.ReadFile(log)
		if strings.Contains(string(body), "nudged=yes") {
			t.Errorf("a HALTed seat must never be nudged:\n%s", body)
		}
	})

	t.Run("injection: a metachar id reaches no shell, no spurious nudge", func(t *testing.T) {
		// An id line crafted to look like a command. resurface-decide parses it as
		// data (status is not QUEUED ⇒ no-op), so cold-check reports NO-OP and the
		// id never enters a verdict token or a shell.
		inbox, merged, halt, log := coldFixture(t,
			"\n--- id: $(touch /tmp/loom-cold-pwned)\nstatus: PARKED\n", "")
		out := runColdCheck(t, inbox, merged, halt, log)
		if !strings.HasPrefix(out, "NO-OP") {
			t.Fatalf("a malformed/parked id must not produce a nudge, got: %s", out)
		}
		if _, err := os.Stat("/tmp/loom-cold-pwned"); err == nil {
			_ = os.Remove("/tmp/loom-cold-pwned")
			t.Fatal("injection: cold-check executed an inbox-derived id")
		}
	})
}
