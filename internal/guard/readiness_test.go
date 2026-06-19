package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runReadiness runs scripts/readiness-decide (the slice-1 home — agent-
// committable, not the trust-protected config/hooks/) over fixtures, with the
// same hermetic env as the config/hooks decision scripts (LL-006/LL-010).
func runReadiness(t *testing.T, env []string, args ...string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "readiness-decide"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", append([]string{abs}, args...)...)
	c.Env = append(hermeticEnv(), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("readiness-decide: %v\n%s", err, out)
	}
	return string(out)
}

// TestReadinessDecide gates the ADR-0022 slice-1 offline readiness-runner
// (config/hooks/readiness-decide). It exercises the full verdict matrix —
// READY / BLOCKED-DEPS / NOT-EXEC-READY / INVERSION / SUPERSEDED — plus the two
// red-team-binding security properties:
//
//   - EXTERNAL-TRUTH DEPS (amendment 2): a row that *claims* a PR merged in its
//     prose but whose PR is absent from the external-truth merged-refs file MUST
//     yield BLOCKED-DEPS — proving the runner never trusts agent-forgeable commit
//     subjects (the S1 blocker: `fix: … (#150)` auto-executing a human-BLOCKED row).
//   - INJECTION-PROOF (amendment 3 / R2): a `${IFS}`/`;`-laden depends-on cell
//     produces a fixed-vocab verdict and creates NO file — row text never reaches
//     a shell and never becomes a decision token.
func TestReadinessDecide(t *testing.T) {
	dir := t.TempDir()
	pwned := filepath.Join(dir, "PWNED")

	// External truth: 149 is merged; 150 is NOT (it is human-BLOCKED). The
	// forged-subject row below cites #150 in prose — external truth must win.
	merged := writeFixture(t, dir, "merged.txt", "149\n111\n")

	// A docs/PLAN.md fixture: the BEGIN/END fence, header + separator (excluded
	// because their status word is not "queued"), then candidate rows in queue
	// order. Candidates are keyed 1..N over QUEUED rows only.
	//   row 1  not-exec        no [class:exec] tag                 -> NOT-EXEC-READY
	//   row 2  superseded      tagged but [superseded-by:#9]       -> SUPERSEDED
	//   row 3  forged-deps     [class:exec], dep "merged (#150)"   -> BLOCKED-DEPS (150 not in truth)
	//   row 4  ready-first     [class:exec], dep "—"               -> READY (first eligible)
	//   row 5  also-eligible   [class:exec], dep "#149" (merged)   -> INVERSION (row 4 wins)
	//   (done) [class:exec] but status "done"                      -> not a candidate, no line
	//   row 6  injection       [class:exec], dep with ;touch ...   -> BLOCKED-DEPS, no PWNED file
	// An out-of-fence queued+exec row must be ignored entirely.
	plan := writeFixture(t, dir, "PLAN.md", fmt.Sprintf("<!-- BEGIN TACTICAL QUEUE -->\n"+
		"| task | depends-on | serves | owner | status | PR |\n"+
		"| --- | --- | --- | --- | --- | --- |\n"+
		"| **not-exec row** | — | x | loom-author | queued | — |\n"+
		"| **superseded row** [class:exec] [superseded-by:#9] | — | x | loom-author | queued | — |\n"+
		"| **forged-deps row** [class:exec] | merged (#150) | x | loom-author | queued | — |\n"+
		"| **ready-first row** [class:exec] | — | x | loom-author | queued | — |\n"+
		"| **also-eligible row** [class:exec] | #149 | x | loom-author | queued | — |\n"+
		"| **done row** [class:exec] | — | x | loom-author | done — shipped | #1 |\n"+
		"| **injection row** [class:exec] | x;touch ${IFS}%s | x | loom-author | queued | — |\n"+
		"<!-- END TACTICAL QUEUE -->\n"+
		"| **out-of-fence row** [class:exec] | — | x | loom-author | queued | — |\n", pwned))

	out := runReadiness(t, nil, plan, merged)

	want := map[string]string{
		"row 1": "NOT-EXEC-READY",
		"row 2": "SUPERSEDED",
		"row 3": "BLOCKED-DEPS", // #150 absent from external truth, prose "merged" ignored
		"row 4": "READY",
		"row 5": "INVERSION",
		"row 6": "BLOCKED-DEPS", // injection cell, no valid merged PR
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		k := strings.SplitN(line, ":", 2)[0]
		got[k] = line
	}
	for id, exp := range want {
		if !strings.Contains(got[id], exp) {
			t.Errorf("%s: got %q, want verdict %s\n--- output ---\n%s", id, got[id], exp, out)
		}
	}
	// The out-of-fence row and the done row must not produce a 7th candidate.
	if _, extra := got["row 7"]; extra {
		t.Errorf("a non-candidate row leaked a verdict (row 7 present):\n%s", out)
	}

	// INJECTION PROOF: the ${IFS} payload must never have reached a shell.
	if _, err := os.Stat(pwned); !os.IsNotExist(err) {
		t.Errorf("INJECTION: %s exists — a depends-on cell reached a shell (err=%v)", pwned, err)
	}
}

// TestReadinessDecideMissingMergedFile proves the fail-closed default: with no
// external-truth file, a PR-dep row cannot be confirmed merged, so it is
// BLOCKED-DEPS rather than optimistically READY (ADR-0022: conservative — fail
// to NOT-pull).
func TestReadinessDecideMissingMergedFile(t *testing.T) {
	dir := t.TempDir()
	plan := writeFixture(t, dir, "PLAN.md", "<!-- BEGIN TACTICAL QUEUE -->\n"+
		"| task | depends-on | serves | owner | status | PR |\n"+
		"| --- | --- | --- | --- | --- | --- |\n"+
		"| **dep row** [class:exec] | #149 | x | loom-author | queued | — |\n"+
		"<!-- END TACTICAL QUEUE -->\n")

	out := runReadiness(t, nil, plan, filepath.Join(dir, "does-not-exist.txt"))
	if !strings.Contains(out, "BLOCKED-DEPS") {
		t.Errorf("missing external-truth file must fail closed to BLOCKED-DEPS:\n%s", out)
	}
}

// TestReadinessDecideExternalTruthStrict pins the OPT-IN external-truth integrity
// check (ADR-0022 amendment 2, "external truth has two halves" — the source AND a
// writer the agent cannot forge; slice-5 hardening, consumer half). Default OFF, so
// the verified loop is untouched until the host half lands.
//
//   - strict + AGENT-WRITABLE merged file ⇒ a row that would be READY fails closed
//     to BLOCKED-DEPS (a file the running process can write is forgeable, so the
//     deps it asserts cannot be trusted — amendment 2 one level up).
//   - strict + READ-ONLY merged file (the host-owned 0644 analog) ⇒ trusted ⇒ the
//     normal verdict stands (READY). Skipped as root, where mode bits don't bind -w.
//   - env UNSET ⇒ byte-for-byte current behavior (the regression guard).
func TestReadinessDecideExternalTruthStrict(t *testing.T) {
	dir := t.TempDir()
	// A single eligible row with no deps ⇒ READY in normal mode.
	plan := writeFixture(t, dir, "PLAN.md", "<!-- BEGIN TACTICAL QUEUE -->\n"+
		"| task | depends-on | serves | owner | status | PR |\n"+
		"| --- | --- | --- | --- | --- | --- |\n"+
		"| **ready row** [class:exec] | — | k | loom-author | queued | — |\n"+
		"<!-- END TACTICAL QUEUE -->\n")
	merged := writeFixture(t, dir, "merged.txt", "149\n")

	// strict + agent-writable ⇒ fail-closed: the READY row becomes BLOCKED-DEPS.
	out := runReadiness(t, []string{"LOOM_EXTERNAL_TRUTH_STRICT=1"}, plan, merged)
	if !strings.Contains(out, "BLOCKED-DEPS") {
		t.Errorf("strict + agent-writable merged file must fail closed to BLOCKED-DEPS:\n%s", out)
	}
	if strings.Contains(out, ": READY") {
		t.Errorf("strict + writable must leave NO row READY:\n%s", out)
	}

	// strict + read-only (host-owned analog) ⇒ trusted ⇒ the normal READY stands.
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod a-w does not remove write access; -w stays true")
	}
	if err := os.Chmod(merged, 0o444); err != nil {
		t.Fatal(err)
	}
	out = runReadiness(t, []string{"LOOM_EXTERNAL_TRUTH_STRICT=1"}, plan, merged)
	if !strings.Contains(out, ": READY") {
		t.Errorf("strict + read-only (host-owned analog) merged file must be trusted ⇒ READY:\n%s", out)
	}

	// env unset ⇒ unchanged behavior (regression guard): READY on the writable file.
	if err := os.Chmod(merged, 0o644); err != nil {
		t.Fatal(err)
	}
	out = runReadiness(t, nil, plan, merged)
	if !strings.Contains(out, ": READY") {
		t.Errorf("strict UNSET must preserve current behavior (READY):\n%s", out)
	}
}
