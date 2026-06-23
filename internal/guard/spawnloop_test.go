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
// plus the caller's overrides. $6 = empty-tick state file (the bounded self-poll
// counter; adv-074 self-wake re-frame).
func runSpawnLoop(t *testing.T, env []string, plan, merged, inbox, ledger, halt, ticks string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "spawn-loop"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs, plan, merged, inbox, ledger, halt, ticks)
	c.Env = append(hermeticEnv(), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("spawn-loop: %v\n%s", err, out)
	}
	return string(out)
}

func readTicks(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// TestSpawnLoop covers the ADR-0022 slice-4 SELF-WAKE matrix (adv-074): HALT-first
// (wakes nothing, refills nothing), the ALLOW path (promotes + WAKE + records one
// grant + resets the empty-tick counter), the self-selection floor (CONFIRM-
// REQUIRED ⇒ NO-WAKE), the rate bound (WAKE-BACKOFF, refills but no grant), and the
// bounded self-poll (WAKE-POLL under the bound, IDLE-STOP at it).
func TestSpawnLoop(t *testing.T) {
	readyExecRow := "| **demo row** [class:exec] | — | agent lifecycle continuity | loom-author | queued | — |"
	allow := []string{"LOOM_AUTOPULL_CLASSES=exec"}
	rateEnv := append([]string{"LOOM_NOW=10000", "LOOM_SPAWN_MAX=3", "LOOM_SPAWN_WINDOW=3600"}, allow...)

	// HALT-before-wake (REQUIRED, ADR-0022): a present HALT ⇒ NO-WAKE HALT, and
	// NOTHING is refilled (no envelope), recorded (no grant), or ticked.
	t.Run("HALT before wake wakes and refills nothing", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := filepath.Join(dir, "ledger")
		halt := writeFixture(t, dir, "HALT", "")
		ticks := filepath.Join(dir, "ticks")
		out := runSpawnLoop(t, allow, plan, merged, inbox, ledger, halt, ticks)
		if !strings.HasPrefix(out, "NO-WAKE HALT") {
			t.Fatalf("HALT must yield NO-WAKE HALT, got: %s", out)
		}
		if strings.Contains(readFile(t, inbox), "--- id: promote-") {
			t.Error("HALT must refill nothing")
		}
		if countLines(t, ledger) != 0 {
			t.Errorf("HALT must record no grant, ledger=%d", countLines(t, ledger))
		}
		if readTicks(t, ticks) != "" {
			t.Errorf("HALT must not touch the tick counter, got %q", readTicks(t, ticks))
		}
	})

	// ALLOW path: READY + allow-listed + rate ok ⇒ WAKE (not SPAWN); promotes
	// QUEUED; records exactly one grant; resets the empty-tick counter to 0.
	t.Run("ALLOW path ⇒ WAKE, promotes QUEUED, one grant, ticks reset", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := filepath.Join(dir, "ledger")
		ticks := writeFixture(t, dir, "ticks", "4\n") // pretend prior empty polls
		noHalt := filepath.Join(dir, "no-halt")
		out := runSpawnLoop(t, allow, plan, merged, inbox, ledger, noHalt, ticks)
		if !strings.HasPrefix(out, "WAKE delay=") {
			t.Fatalf("want WAKE delay= on the allow path, got: %s", out)
		}
		if strings.HasPrefix(out, "WAKE-") {
			t.Fatalf("ALLOW must be plain WAKE, not WAKE-POLL/BACKOFF: %s", out)
		}
		if !strings.Contains(readFile(t, inbox), "status: QUEUED") {
			t.Error("allow-listed READY row must be promoted QUEUED")
		}
		if n := countLines(t, ledger); n != 1 {
			t.Errorf("WAKE must record exactly one grant, ledger=%d", n)
		}
		if readTicks(t, ticks) != "0" {
			t.Errorf("finding work must reset the empty-tick counter to 0, got %q", readTicks(t, ticks))
		}
	})

	// Built-but-unmerged steady state (the trial's headline loop finding): a READY
	// row whose envelope is already worked (DONE) with nothing QUEUED. promote-next
	// is idempotent ("already present"), but spawn-loop must NOT WAKE into it —
	// readiness-decide reads main's PLAN, which shows a built row as "queued" until
	// its PR merges, so an agent that cannot push would otherwise emit WAKE forever.
	// It is an empty tick: WAKE-POLL under the bound (so the loop self-terminates via
	// IDLE-STOP), never plain WAKE, and records no grant.
	t.Run("already-built row, nothing QUEUED ⇒ WAKE-POLL not WAKE", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		// the READY row's envelope already exists and is DONE; nothing is QUEUED.
		inbox := writeFixture(t, dir, "inbox.md", "--- id: promote-demo-row\nstatus: DONE\n")
		ledger := filepath.Join(dir, "ledger")
		ticks := writeFixture(t, dir, "ticks", "1\n")
		noHalt := filepath.Join(dir, "no-halt")
		out := runSpawnLoop(t, allow, plan, merged, inbox, ledger, noHalt, ticks)
		if !strings.HasPrefix(out, "WAKE-POLL") {
			t.Fatalf("an already-built row with nothing QUEUED must be WAKE-POLL (empty tick), not WAKE: %s", out)
		}
		if readTicks(t, ticks) != "2" {
			t.Errorf("WAKE-POLL must increment the empty-tick counter to 2, got %q", readTicks(t, ticks))
		}
		if countLines(t, ledger) != 0 {
			t.Errorf("a non-WAKE tick must record no grant, ledger=%d", countLines(t, ledger))
		}
	})

	t.Run("off-allow-list READY ⇒ NO-WAKE CONFIRM-REQUIRED", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		out := runSpawnLoop(t, nil, plan, merged, inbox, filepath.Join(dir, "ledger"),
			filepath.Join(dir, "no-halt"), filepath.Join(dir, "ticks"))
		if !strings.HasPrefix(out, "NO-WAKE CONFIRM-REQUIRED") {
			t.Fatalf("want NO-WAKE CONFIRM-REQUIRED, got: %s", out)
		}
		if !strings.Contains(readFile(t, inbox), "status: CONFIRM-REQUIRED") {
			t.Error("the row should still be refilled as CONFIRM-REQUIRED (human tier)")
		}
	})

	// Rate bound: a ledger already at LOOM_SPAWN_MAX in-window ⇒ WAKE-BACKOFF. The
	// refill still happens (promote runs before the gate); no grant is recorded.
	t.Run("at rate limit ⇒ WAKE-BACKOFF, refills but records no grant", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, readyExecRow)
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ledger := writeFixture(t, dir, "ledger", "9000\n9001\n9002\n")
		out := runSpawnLoop(t, rateEnv, plan, merged, inbox, ledger,
			filepath.Join(dir, "no-halt"), filepath.Join(dir, "ticks"))
		if !strings.HasPrefix(out, "WAKE-BACKOFF") {
			t.Fatalf("want WAKE-BACKOFF at the limit, got: %s", out)
		}
		if !strings.Contains(readFile(t, inbox), "status: QUEUED") {
			t.Error("the refill runs before the rate gate — envelope should be minted")
		}
		if n := countLines(t, ledger); n != 3 {
			t.Errorf("a rate-bounded cycle must append no grant, ledger=%d (want 3)", n)
		}
	})

	// Bounded self-poll: no READY backlog under the give-up bound ⇒ WAKE-POLL and
	// the empty-tick counter increments.
	t.Run("no backlog under bound ⇒ WAKE-POLL, ticks increment", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, "| **untagged** | — | k | loom-author | queued | — |")
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ticks := writeFixture(t, dir, "ticks", "1\n")
		out := runSpawnLoop(t, append([]string{"LOOM_WAKE_MAX_EMPTY=3"}, allow...),
			plan, merged, inbox, filepath.Join(dir, "ledger"), filepath.Join(dir, "no-halt"), ticks)
		if !strings.HasPrefix(out, "WAKE-POLL delay=") {
			t.Fatalf("want WAKE-POLL under the bound, got: %s", out)
		}
		if readTicks(t, ticks) != "2" {
			t.Errorf("an empty poll must increment the tick counter to 2, got %q", readTicks(t, ticks))
		}
	})

	// Give-up: at the bound, NO re-arm ⇒ IDLE-STOP, and the counter resets so a
	// future external wake gets a fresh poll budget.
	t.Run("no backlog at bound ⇒ IDLE-STOP, ticks reset", func(t *testing.T) {
		dir := t.TempDir()
		plan := planFixture(t, dir, "| **untagged** | — | k | loom-author | queued | — |")
		merged := writeFixture(t, dir, "merged.txt", "")
		inbox := writeFixture(t, dir, "inbox.md", "")
		ticks := writeFixture(t, dir, "ticks", "2\n") // +1 ⇒ 3 == MAX
		out := runSpawnLoop(t, append([]string{"LOOM_WAKE_MAX_EMPTY=3"}, allow...),
			plan, merged, inbox, filepath.Join(dir, "ledger"), filepath.Join(dir, "no-halt"), ticks)
		if !strings.HasPrefix(out, "IDLE-STOP") {
			t.Fatalf("want IDLE-STOP at the give-up bound, got: %s", out)
		}
		if readTicks(t, ticks) != "0" {
			t.Errorf("give-up must reset the tick counter to 0, got %q", readTicks(t, ticks))
		}
	})
}

// TestSpawnLoopCadenceClamp pins the cadence clamp (adv-074 bound): an out-of-range
// idle delay is pulled into the ScheduleWakeup [60,3600] window — never a <60s spin.
func TestSpawnLoopCadenceClamp(t *testing.T) {
	dir := t.TempDir()
	plan := planFixture(t, dir, "| **untagged** | — | k | loom-author | queued | — |")
	merged := writeFixture(t, dir, "merged.txt", "")
	inbox := writeFixture(t, dir, "inbox.md", "")
	noHalt := filepath.Join(dir, "no-halt")

	huge := runSpawnLoop(t, []string{"LOOM_WAKE_IDLE_DELAY=99999"}, plan, merged, inbox,
		filepath.Join(dir, "ledger"), noHalt, filepath.Join(dir, "t1"))
	if !strings.Contains(huge, "delay=3600") {
		t.Errorf("an over-range idle delay must clamp to 3600, got: %s", huge)
	}
	tiny := runSpawnLoop(t, []string{"LOOM_WAKE_IDLE_DELAY=1"}, plan, merged, inbox,
		filepath.Join(dir, "ledger"), noHalt, filepath.Join(dir, "t2"))
	if !strings.Contains(tiny, "delay=60") {
		t.Errorf("an under-range idle delay must clamp to 60 (no fast spin), got: %s", tiny)
	}
}

// TestSpawnLoopFixedVocab pins injection-safety: the verdict leads with a hardcoded
// token even when a row carries shell metacharacters — row prose never reaches a
// shell or the verdict token.
func TestSpawnLoopFixedVocab(t *testing.T) {
	dir := t.TempDir()
	canary := filepath.Join(dir, "canary")
	plan := planFixture(t, dir,
		"| **evil $(touch "+canary+") row** [class:exec] | — | k | loom-author | queued | — |")
	merged := writeFixture(t, dir, "merged.txt", "")
	inbox := writeFixture(t, dir, "inbox.md", "")
	out := runSpawnLoop(t, []string{"LOOM_AUTOPULL_CLASSES=exec"}, plan, merged, inbox,
		filepath.Join(dir, "ledger"), filepath.Join(dir, "no-halt"), filepath.Join(dir, "ticks"))
	lead := strings.Fields(out)
	allowed := map[string]bool{"WAKE": true, "WAKE-POLL": true, "WAKE-BACKOFF": true, "IDLE-STOP": true, "NO-WAKE": true}
	if len(lead) == 0 || !allowed[lead[0]] {
		t.Fatalf("verdict must lead with a fixed token, got: %s", out)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("injection: a row title was eval'd (canary file created)")
	}
}
