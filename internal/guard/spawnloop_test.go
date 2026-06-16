package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSpawnLoop runs scripts/spawn-loop (ADR-0022 slice-4 home — agent-committable
// scripts/, same as slices 1–3) over fixtures with hermetic env (LL-006/LL-010)
// plus the caller's overrides (LOOM_AUTOPULL_CLASSES for the self-selection floor;
// LOOM_NOW/MAX/WINDOW for deterministic rate tests).
func runSpawnLoop(t *testing.T, env []string, plan, merged, inbox, ledger, halt string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "spawn-loop"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs, plan, merged, inbox, ledger, halt)
	c.Env = append(hermeticEnv(), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("spawn-loop: %v\n%s", err, out)
	}
	return string(out)
}

// TestSpawnLoop covers the ADR-0022 slice-4 orchestrator matrix: HALT-before-spawn
// (REQUIRED — spawns nothing AND refills nothing), no READY backlog, the
// self-selection floor (CONFIRM-REQUIRED ⇒ no spawn), the ALLOW path (promotes +
// records exactly one grant), and the rate bound (refills but does not spawn).
func TestSpawnLoop(t *testing.T) {
	readyExecRow := "| **demo row** [class:exec] | — | agent lifecycle continuity | loom-author | queued | — |"
	allow := []string{"LOOM_AUTOPULL_CLASSES=exec"}
	// Deterministic rate clock: now=10000, window=3600 ⇒ in-window threshold 6400.
	rateEnv := append([]string{"LOOM_NOW=10000", "LOOM_SPAWN_MAX=3", "LOOM_SPAWN_WINDOW=3600"}, allow...)

	// HALT-before-spawn (REQUIRED, ADR-0022): a present HALT sentinel ⇒ NO-SPAWN
	// HALT, and NOTHING is refilled (no envelope) or recorded (no ledger line) —
	// the HALT short-circuits BEFORE promote-next/spawn-guard.
	t.Run("HALT before spawn spawns and refills nothing", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := filepath.Join(dir, "ledger")
		halt := writeFixture(t, dir, "HALT", "")
		out := runSpawnLoop(t, allow, plan, merged, inbox, ledger, halt)
		if !strings.HasPrefix(out, "NO-SPAWN HALT") {
			t.Fatalf("HALT must yield NO-SPAWN HALT, got: %s", out)
		}
		if body := readFile(t, inbox); strings.Contains(body, "--- id: promote-") {
			t.Errorf("HALT must refill nothing, but an envelope was minted:\n%s", body)
		}
		if countLines(t, ledger) != 0 {
			t.Errorf("HALT must record no spawn grant, ledger has %d lines", countLines(t, ledger))
		}
	})

	t.Run("no READY backlog ⇒ NO-SPAWN NO-BACKLOG", func(t *testing.T) {
		dir := t.TempDir()
		// A candidate row WITHOUT [class:exec] is NOT-EXEC-READY ⇒ never READY.
		plan := planFixture(t, dir, "| **untagged** | — | k | loom-author | queued | — |")
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		noHalt := filepath.Join(dir, "no-halt")
		out := runSpawnLoop(t, allow, plan, merged, inbox, filepath.Join(dir, "ledger"), noHalt)
		if !strings.HasPrefix(out, "NO-SPAWN NO-BACKLOG") {
			t.Fatalf("want NO-SPAWN NO-BACKLOG, got: %s", out)
		}
	})

	// Self-selection floor: a READY row whose class is NOT on the auto-pull
	// allow-list mints CONFIRM-REQUIRED ⇒ NO-SPAWN (human tier), and never spawns.
	t.Run("off-allow-list READY ⇒ NO-SPAWN CONFIRM-REQUIRED, no grant", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := filepath.Join(dir, "ledger")
		noHalt := filepath.Join(dir, "no-halt")
		out := runSpawnLoop(t, nil, plan, merged, inbox, ledger, noHalt) // empty allow-list
		if !strings.HasPrefix(out, "NO-SPAWN CONFIRM-REQUIRED") {
			t.Fatalf("want NO-SPAWN CONFIRM-REQUIRED, got: %s", out)
		}
		if !strings.Contains(readFile(t, inbox), "status: CONFIRM-REQUIRED") {
			t.Error("the row should still be refilled as CONFIRM-REQUIRED (human tier)")
		}
		if countLines(t, ledger) != 0 {
			t.Errorf("a confirm-required promote must record no spawn grant, ledger=%d", countLines(t, ledger))
		}
	})

	// ALLOW path: READY + allow-listed + rate under bound ⇒ SPAWN; the row is
	// promoted QUEUED and the grant is recorded as exactly one ledger line.
	t.Run("ALLOW path ⇒ SPAWN, promotes QUEUED, records one grant", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := filepath.Join(dir, "ledger")
		noHalt := filepath.Join(dir, "no-halt")
		out := runSpawnLoop(t, allow, plan, merged, inbox, ledger, noHalt)
		if !strings.HasPrefix(out, "SPAWN") {
			t.Fatalf("want SPAWN on the allow path, got: %s", out)
		}
		if !strings.Contains(readFile(t, inbox), "status: QUEUED") {
			t.Error("allow-listed READY row must be promoted QUEUED")
		}
		if n := countLines(t, ledger); n != 1 {
			t.Errorf("SPAWN must record exactly one grant, ledger has %d lines", n)
		}
	})

	// Rate bound: a ledger already at LOOM_SPAWN_MAX in-window ⇒ NO-SPAWN RATE.
	// The refill still happens (promote runs before the gate) but no grant is
	// recorded — the durable cross-spawn bound holds.
	t.Run("at rate limit ⇒ NO-SPAWN RATE, refills but records no grant", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := writeFixture(t, dir, "ledger", "9000\n9001\n9002\n") // 3 in-window = MAX
		noHalt := filepath.Join(dir, "no-halt")
		out := runSpawnLoop(t, rateEnv, plan, merged, inbox, ledger, noHalt)
		if !strings.HasPrefix(out, "NO-SPAWN RATE") {
			t.Fatalf("want NO-SPAWN RATE at the limit, got: %s", out)
		}
		if !strings.Contains(readFile(t, inbox), "status: QUEUED") {
			t.Error("the refill (promote-next) runs before the rate gate — envelope should be minted")
		}
		if n := countLines(t, ledger); n != 3 {
			t.Errorf("a rate-denied cycle must append no grant, ledger has %d lines (want 3)", n)
		}
	})
}

// TestSpawnLoopFixedVocab pins injection-safety: every verdict's leading token is
// a hardcoded constant from the fixed set, even when a row carries shell
// metacharacters — row prose never reaches a shell or the verdict token.
func TestSpawnLoopFixedVocab(t *testing.T) {
	dir := t.TempDir()
	canary := filepath.Join(dir, "canary")
	// A malicious task title: if any field were eval'd, the canary file appears.
	plan := planFixture(t, dir,
		"| **evil $(touch "+canary+") row** [class:exec] | — | k | loom-author | queued | — |")
	merged := writeFixture(t, dir, "merged.txt", "")
	inbox := writeFixture(t, dir, "inbox.md", "")
	noHalt := filepath.Join(dir, "no-halt")
	out := runSpawnLoop(t, []string{"LOOM_AUTOPULL_CLASSES=exec"}, plan, merged, inbox, filepath.Join(dir, "ledger"), noHalt)
	lead := strings.Fields(out)
	if len(lead) == 0 || (lead[0] != "SPAWN" && lead[0] != "NO-SPAWN") {
		t.Fatalf("verdict must lead with a fixed token (SPAWN/NO-SPAWN), got: %s", out)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("injection: a row title was eval'd by the loop (canary file created)")
	}
}
