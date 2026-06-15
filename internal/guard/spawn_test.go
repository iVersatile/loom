package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSpawnGuard runs scripts/spawn-guard (slice-3 home — agent-committable
// scripts/, same as slices 1–2) over fixtures with hermetic env (LL-006/LL-010)
// plus the caller's overrides (LOOM_NOW / LOOM_SPAWN_MAX / LOOM_SPAWN_WINDOW for
// deterministic rate tests).
func runSpawnGuard(t *testing.T, env []string, ledger, halt string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "spawn-guard"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs, ledger, halt)
	c.Env = append(hermeticEnv(), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("spawn-guard: %v\n%s", err, out)
	}
	return string(out)
}

// countLines counts newline-terminated lines (fixtures + appended grants always
// end with "\n", so a plain newline count is exact: "" ⇒ 0, "9000\n" ⇒ 1).
func countLines(t *testing.T, p string) int {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

// TestSpawnGuard covers the ADR-0022 slice-3 decision matrix + properties:
// ALLOW under limit (records exactly one line) · DENY-RATE at/over limit (no
// append) · DENY-HALT (beats an under-limit ALLOW, no append) · window aging ·
// fail-closed on a corrupt ledger · injection (no shell, no file).
func TestSpawnGuard(t *testing.T) {
	// Deterministic clock: now=10000, window=3600 ⇒ in-window threshold = 6400.
	base := []string{"LOOM_NOW=10000", "LOOM_SPAWN_MAX=3", "LOOM_SPAWN_WINDOW=3600"}
	noHalt := filepath.Join(t.TempDir(), "no-such-halt")

	t.Run("ALLOW under limit records exactly one line", func(t *testing.T) {
		dir := t.TempDir()
		ledger := writeFixture(t, dir, "ledger", "9000\n") // 1 in-window < 3
		out := runSpawnGuard(t, base, ledger, noHalt)
		if !strings.Contains(out, "ALLOW") {
			t.Fatalf("want ALLOW under limit, got: %s", out)
		}
		if n := countLines(t, ledger); n != 2 {
			t.Errorf("ALLOW must append exactly one line: got %d lines, want 2", n)
		}
	})

	t.Run("missing ledger is the legitimate first spawn", func(t *testing.T) {
		dir := t.TempDir()
		ledger := filepath.Join(dir, "ledger") // does not exist yet
		out := runSpawnGuard(t, base, ledger, noHalt)
		if !strings.Contains(out, "ALLOW") {
			t.Fatalf("missing ledger (no prior spawns) must ALLOW, got: %s", out)
		}
		if n := countLines(t, ledger); n != 1 {
			t.Errorf("first spawn must create a 1-line ledger: got %d", n)
		}
	})

	t.Run("DENY-RATE at limit, no append", func(t *testing.T) {
		dir := t.TempDir()
		ledger := writeFixture(t, dir, "ledger", "9000\n9500\n9900\n") // 3 in-window == max
		out := runSpawnGuard(t, base, ledger, noHalt)
		if !strings.Contains(out, "DENY-RATE") {
			t.Fatalf("want DENY-RATE at limit, got: %s", out)
		}
		if !strings.Contains(out, "retry in ~2600s") {
			t.Errorf("want bounded backoff ~2600s (oldest 9000 + 3600 - 10000), got: %s", out)
		}
		if n := countLines(t, ledger); n != 3 {
			t.Errorf("DENY must NOT append: got %d lines, want 3", n)
		}
	})

	t.Run("window aging: out-of-window entries do not count", func(t *testing.T) {
		dir := t.TempDir()
		ledger := writeFixture(t, dir, "ledger", "1000\n2000\n3000\n") // all < 6400 threshold
		out := runSpawnGuard(t, base, ledger, noHalt)
		if !strings.Contains(out, "ALLOW") {
			t.Fatalf("aged-out entries must not bar a spawn, got: %s", out)
		}
	})

	t.Run("DENY-HALT beats an under-limit ALLOW, no append", func(t *testing.T) {
		dir := t.TempDir()
		ledger := writeFixture(t, dir, "ledger", "") // empty ⇒ would be ALLOW
		halt := writeFixture(t, dir, "HALT", "")
		out := runSpawnGuard(t, base, ledger, halt)
		if !strings.Contains(out, "DENY-HALT") {
			t.Fatalf("HALT present must DENY-HALT even under limit, got: %s", out)
		}
		if n := countLines(t, ledger); n != 0 {
			t.Errorf("DENY-HALT must NOT append: got %d lines, want 0", n)
		}
	})

	t.Run("fail-closed on a corrupt ledger, no append", func(t *testing.T) {
		dir := t.TempDir()
		ledger := writeFixture(t, dir, "ledger", "9000\nnot-an-epoch\n") // malformed line
		out := runSpawnGuard(t, base, ledger, noHalt)
		if !strings.Contains(out, "DENY-RATE") {
			t.Fatalf("a malformed ledger must fail closed to DENY-RATE, got: %s", out)
		}
		if n := countLines(t, ledger); n != 2 {
			t.Errorf("fail-closed must NOT append: got %d lines, want 2", n)
		}
	})

	t.Run("injection: a metachar ledger line reaches no shell", func(t *testing.T) {
		dir := t.TempDir()
		pwned := filepath.Join(dir, "PWNED")
		ledger := writeFixture(t, dir, "ledger", "9000\n;touch ${IFS}"+pwned+"\n")
		out := runSpawnGuard(t, base, ledger, noHalt)
		if !strings.Contains(out, "DENY-RATE") {
			t.Fatalf("a metachar line is malformed ⇒ DENY-RATE, got: %s", out)
		}
		if _, err := os.Stat(pwned); !os.IsNotExist(err) {
			t.Errorf("INJECTION: %s exists — a ledger line reached a shell (err=%v)", pwned, err)
		}
	})
}
