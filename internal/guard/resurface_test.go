package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests gate the human-applied drain-hook diff (ADR-0020, PR #137): they
// exercise the shared decision scripts the trust-path drain will source —
// config/hooks/resurface-decide (the PARK/re-surface/escalate decision) and
// config/hooks/pull-next (the priority-inversion-safe picker). Green here is the
// signal the prepared hook is safe to apply.

// runScript runs a config/hooks script over fixture files and returns combined
// output. Hermetic env (LL-006/LL-010) plus the explicit overrides the caller
// passes (LOOM_NOW / LOOM_MAX_PARK_AGE for deterministic age tests).
func runScript(t *testing.T, name string, env []string, args ...string) string {
	t.Helper()
	c := exec.Command("sh", append([]string{absHook(t, name)}, args...)...)
	c.Env = append(hermeticEnv(), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
	return string(out)
}

func writeFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestResurfaceDecide covers the full ADR-0020 decision matrix AND the R2
// injection property: deliver / skip-parked / resurface-on-clear / over-age
// ESCALATE / malformed ESCALATE / skip-superseded, plus a ${IFS} shell-injection
// payload in parked-on that must create no file and fail-closed.
func TestResurfaceDecide(t *testing.T) {
	dir := t.TempDir()
	pwned := filepath.Join(dir, "PWNED")
	merged := writeFixture(t, dir, "merged.txt", "130\n131\n132\n")
	// Header fields PRECEDE status: (the schema contract); t-LATE deliberately
	// violates it to prove a post-status predicate is ignored → malformed ESCALATE.
	inbox := writeFixture(t, dir, "inbox.md", fmt.Sprintf(`--- id: t-deliver
status: QUEUED

--- id: t-wait
parked-on: pr-merged:999
parked-at: 1000
status: PARKED

--- id: t-clear
parked-on: pr-merged:132
parked-at: 1000
status: PARKED

--- id: t-old
parked-on: pr-merged:999
parked-at: 100
status: PARKED

--- id: t-inj
parked-on: exists:x;touch${IFS}%s
parked-at: 1000
status: PARKED

--- id: t-late
status: PARKED
parked-on: pr-merged:132

--- id: t-sup
superseded-by: t-deliver
status: QUEUED
`, pwned))

	env := []string{"LOOM_NOW=1500", "LOOM_MAX_PARK_AGE=500"}
	out := runScript(t, "resurface-decide", env, inbox, merged)

	want := map[string]string{
		"t-deliver": "DELIVER",
		"t-wait":    "SKIP-PARKED",
		"t-clear":   "RESURFACE",
		"t-old":     "ESCALATE",    // age 1400 > 500
		"t-inj":     "SKIP-PARKED", // fail-closed, never resurface on an opaque predicate
		"t-late":    "ESCALATE",    // parked-on after status → not parsed → malformed
		"t-sup":     "SKIP-SUPERSEDED",
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		id := strings.SplitN(line, ":", 2)[0]
		if exp, ok := want[id]; ok {
			if !strings.Contains(line, exp) {
				t.Errorf("%s: got %q, want decision %s", id, line, exp)
			}
			delete(want, id)
		}
	}
	for id, exp := range want {
		t.Errorf("missing decision line for %s (want %s)\n--- output ---\n%s", id, exp, out)
	}

	// R2 PROOF: the ${IFS} payload must NOT have reached a shell.
	if _, err := os.Stat(pwned); !os.IsNotExist(err) {
		t.Errorf("INJECTION: %s exists — the predicate reached a shell (err=%v)", pwned, err)
	}
}

// TestPullNextNoPriorityInversion covers ADR-0020/R5: pull-next picks the first
// independent eligible item and SKIPS a QUEUED item that depends on a non-DONE
// (e.g. PARKED) item; an all-blocked inbox yields no pick (stay idle / escalate).
func TestPullNextNoPriorityInversion(t *testing.T) {
	dir := t.TempDir()
	inbox := writeFixture(t, dir, "inbox.md", `--- id: p-blocked
parked-on: pr-merged:999
status: PARKED

--- id: p-dependent
depends-on: p-blocked
status: QUEUED

--- id: p-free
status: QUEUED

--- id: p-done
status: DONE
`)
	out := runScript(t, "pull-next", nil, inbox)
	if !strings.Contains(out, "skip p-dependent") {
		t.Errorf("pull-next must skip the item depending on a PARKED one:\n%s", out)
	}
	if !strings.Contains(out, "PICK p-free") {
		t.Errorf("pull-next must pick the independent item p-free:\n%s", out)
	}

	// All-blocked: the only QUEUED item depends on a PARKED one → no pick.
	blocked := writeFixture(t, dir, "blocked.md", `--- id: q-parked
parked-on: pr-merged:999
status: PARKED

--- id: q-dep
depends-on: q-parked
status: QUEUED
`)
	out = runScript(t, "pull-next", nil, blocked)
	if !strings.Contains(out, "PICK (none") {
		t.Errorf("all-blocked inbox must yield no pick (stay idle / escalate):\n%s", out)
	}
}
