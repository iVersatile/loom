package guard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runSelfWakeDoctor runs scripts/self-wake-doctor against a seat root, returning
// its combined output and exit code. Reuses hermeticEnv() (block_test.go) so the
// fixture process can only ever touch the root it is handed. The doctor is
// read-only, so unlike runSelfWakeTick a non-zero exit is EXPECTED (a FAIL verdict)
// rather than a test failure — we return the code for the caller to assert.
func runSelfWakeDoctor(t *testing.T, root string, env ...string) (string, int) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "self-wake-doctor"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs)
	c.Env = append(append(hermeticEnv(), "LOOM_ROOT="+root), env...)
	out, err := c.CombinedOutput()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("self-wake-doctor: unexpected exec error: %v\n%s", err, out)
		}
		code = ee.ExitCode()
	}
	return string(out), code
}

// snapshotInbox reads every file under <root>/.scratch/inbox so a test can assert
// byte-for-byte that a doctor run mutated NOTHING (no envelope minted, no tick/
// ledger write). The returned map is keyed by filename.
func snapshotInbox(t *testing.T, root string) map[string][]byte {
	t.Helper()
	dir := filepath.Join(root, ".scratch", "inbox")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string][]byte{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = b
	}
	return m
}

const execRow = "| **demo** [class:exec] | — | agent lifecycle | loom-author | queued | — |"

// TestSelfWakeDoctorReadOnly is the POINT of the doctor: it diagnoses the self-wake
// wiring WITHOUT actuating it. A run must leave the seat inbox + every .scratch
// state file byte-identical and mint no envelope (only self-wake-tick/promote-next
// mutate; the doctor invokes ONLY the pure-read readiness-decide).
func TestSelfWakeDoctorReadOnly(t *testing.T) {
	root := selfWakeRoot(t, execRow)
	before := snapshotInbox(t, root)

	out, code := runSelfWakeDoctor(t, root, "LOOM_AUTOPULL_CLASSES=exec")
	if code != 0 {
		t.Fatalf("healthy root must exit 0, got %d:\n%s", code, out)
	}

	after := snapshotInbox(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("doctor mutated the inbox tree (must be read-only)\nbefore=%v\nafter=%v", keys(before), keys(after))
	}
	// No new files (a minted envelope, a written .spawn-log/.wake-ticks would all
	// appear as new keys; DeepEqual above catches content drift on existing ones).
	if len(after) != len(before) {
		t.Errorf("doctor created files under .scratch/inbox: before=%v after=%v", keys(before), keys(after))
	}
	for _, f := range []string{".spawn-log", ".wake-ticks"} {
		if _, ok := after[f]; ok {
			t.Errorf("doctor must not create %s (mutating state)", f)
		}
	}
}

// TestSelfWakeDoctorExit pins the exit contract: 0 when wired soundly, 1 on a hard
// break (the load-bearing backlog source or inbox absent).
func TestSelfWakeDoctorExit(t *testing.T) {
	t.Run("healthy ⇒ exit 0", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, code := runSelfWakeDoctor(t, root)
		if code != 0 {
			t.Fatalf("want exit 0, got %d:\n%s", code, out)
		}
		if !strings.Contains(out, "VERDICT: OK") {
			t.Errorf("want VERDICT: OK, got:\n%s", out)
		}
	})

	t.Run("absent PLAN ⇒ FAIL exit 1", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, code := runSelfWakeDoctor(t, root, "LOOM_WAKE_PLAN="+filepath.Join(root, "docs", "NOPE.md"))
		if code != 1 {
			t.Fatalf("absent PLAN must exit 1, got %d:\n%s", code, out)
		}
		if !strings.Contains(out, "FAIL  path:plan") || !strings.Contains(out, "VERDICT: FAIL") {
			t.Errorf("want FAIL path:plan + VERDICT: FAIL, got:\n%s", out)
		}
	})

	t.Run("absent inbox ⇒ FAIL exit 1", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, code := runSelfWakeDoctor(t, root, "LOOM_WAKE_INBOX="+filepath.Join(root, ".scratch", "inbox", "nope.md"))
		if code != 1 {
			t.Fatalf("absent inbox must exit 1, got %d:\n%s", code, out)
		}
		if !strings.Contains(out, "FAIL  path:inbox") {
			t.Errorf("want FAIL path:inbox, got:\n%s", out)
		}
	})
}

// TestSelfWakeDoctorGates exercises the gate branches under fixtures so the doctor
// is not all-green theatre — each gate that would steer the live verdict must
// surface its WARN. Gates are WARN (a deliberate human control / soft caveat), not
// FAIL, so all of these still exit 0.
func TestSelfWakeDoctorGates(t *testing.T) {
	inboxFile := func(root, name, body string) {
		if err := os.WriteFile(filepath.Join(root, ".scratch", "inbox", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("HALT ⇒ WARN", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		inboxFile(root, "HALT", "")
		out, code := runSelfWakeDoctor(t, root)
		mustWarn(t, out, code, "gate:halt", "HALT sentinel PRESENT")
	})

	t.Run("empty-tick floor ⇒ OK 0/5", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, _ := runSelfWakeDoctor(t, root)
		if !strings.Contains(out, "OK    gate:empty-ticks    0/5") {
			t.Errorf("want OK gate:empty-ticks 0/5, got:\n%s", out)
		}
	})

	t.Run("near MAX_EMPTY ⇒ WARN", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		inboxFile(root, ".wake-ticks", "4\n")
		out, code := runSelfWakeDoctor(t, root)
		mustWarn(t, out, code, "gate:empty-ticks", "one empty poll from IDLE-STOP")
	})

	t.Run("at MAX_EMPTY ⇒ WARN IDLE-STOP", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		inboxFile(root, ".wake-ticks", "5\n")
		out, code := runSelfWakeDoctor(t, root)
		mustWarn(t, out, code, "gate:empty-ticks", "IDLE-STOP")
	})

	t.Run("rate at SPAWN_MAX ⇒ WARN", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		// Three grants inside the window relative to a pinned NOW ⇒ 3/3 at-limit.
		inboxFile(root, ".spawn-log", "1000000\n1000000\n1000000\n")
		out, code := runSelfWakeDoctor(t, root, "LOOM_NOW=1000000")
		mustWarn(t, out, code, "gate:rate", "at limit")
	})

	t.Run("autopull-floor empty ⇒ WARN", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, code := runSelfWakeDoctor(t, root) // no LOOM_AUTOPULL_CLASSES => floor closed
		mustWarn(t, out, code, "gate:autopull-floor", "CLOSED")
	})

	t.Run("autopull-floor set ⇒ OK", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, code := runSelfWakeDoctor(t, root, "LOOM_AUTOPULL_CLASSES=exec")
		if code != 0 {
			t.Fatalf("want exit 0, got %d:\n%s", code, out)
		}
		line := ""
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "gate:autopull-floor") {
				line = l
				break
			}
		}
		if !strings.HasPrefix(line, "OK") || !strings.Contains(line, "exec") {
			t.Errorf("want OK gate:autopull-floor containing exec, got line: %q\nfull:\n%s", line, out)
		}
	})
}

// TestSelfWakeDoctorReadiness pins ADD 1: the readiness section invokes the pure-
// read readiness-decide and surfaces the trial's headline signal — a [class:exec]
// row with cleared deps reports READY and the pullable-work summary.
func TestSelfWakeDoctorReadiness(t *testing.T) {
	t.Run("class:exec eligible ⇒ READY + pullable", func(t *testing.T) {
		root := selfWakeRoot(t, execRow)
		out, code := runSelfWakeDoctor(t, root)
		if code != 0 {
			t.Fatalf("want exit 0, got %d:\n%s", code, out)
		}
		if !strings.Contains(out, "OK    readiness           row 1: READY") {
			t.Errorf("want a READY row, got:\n%s", out)
		}
		if !strings.Contains(out, "READY — pullable work exists") {
			t.Errorf("want the pullable-work summary, got:\n%s", out)
		}
	})

	t.Run("no [class:exec] tag ⇒ NOT-EXEC-READY, none pullable", func(t *testing.T) {
		root := selfWakeRoot(t, "| **untagged** | — | design stub | loom-author | queued | — |")
		out, _ := runSelfWakeDoctor(t, root)
		if !strings.Contains(out, "NOT-EXEC-READY") {
			t.Errorf("want NOT-EXEC-READY for an untagged row, got:\n%s", out)
		}
		if !strings.Contains(out, "no READY row") {
			t.Errorf("want the no-pullable-work summary, got:\n%s", out)
		}
	})
}

func mustWarn(t *testing.T, out string, code int, area, want string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("gate WARN must still exit 0, got %d:\n%s", code, out)
	}
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, area) {
			line = l
			break
		}
	}
	if !strings.HasPrefix(line, "WARN") || !strings.Contains(line, want) {
		t.Errorf("want WARN %s containing %q, got line: %q\nfull:\n%s", area, want, line, out)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
