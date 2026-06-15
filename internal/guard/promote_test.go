package guard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runPromote runs scripts/promote-next (slice-2 home, same agent-committable
// scripts/ as slice 1) over fixtures, hermetic env (LL-006/LL-010) plus the
// caller's overrides (LOOM_AUTOPULL_CLASSES for the self-selection floor).
func runPromote(t *testing.T, env []string, plan, merged, inbox string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "scripts", "promote-next"))
	if err != nil {
		t.Fatal(err)
	}
	c := exec.Command("sh", abs, plan, merged, inbox)
	c.Env = append(hermeticEnv(), env...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("promote-next: %v\n%s", err, out)
	}
	return string(out)
}

// planFixture wraps queue rows in the BEGIN/END TACTICAL QUEUE fence + header
// (the header/separator are excluded as candidates — their status word isn't
// "queued"), matching the real docs/PLAN.md shape readiness-decide parses.
func planFixture(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("<!-- BEGIN TACTICAL QUEUE -->\n")
	b.WriteString("| task | depends-on | serves | owner | status | PR |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	b.WriteString("<!-- END TACTICAL QUEUE -->\n")
	return writeFixture(t, dir, "PLAN.md", b.String())
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestPromoteNextOnlyReadyMints (case a): only a READY verdict mints. A backlog
// whose sole candidate is BLOCKED-DEPS / NOT-EXEC-READY produces no envelope;
// an INVERSION (non-first eligible) row is never the one minted.
func TestPromoteNextOnlyReadyMints(t *testing.T) {
	dir := t.TempDir()
	merged := writeFixture(t, dir, "merged.txt", "149\n")

	// No eligible row at all: blocked deps + untagged ⇒ no READY ⇒ no mint.
	plan := planFixture(t, dir,
		"| **blocked row** [class:exec] | #999 | a | loom-author | queued | — |",
		"| **untagged row** | — | b | loom-author | queued | — |",
	)
	inbox := writeFixture(t, dir, "inbox.md", "")
	out := runPromote(t, nil, plan, merged, inbox)
	if !strings.Contains(out, "nothing to promote") {
		t.Errorf("no eligible row must mint nothing:\n%s", out)
	}
	if strings.Contains(readFile(t, inbox), "--- id: promote-") {
		t.Errorf("an envelope was minted from a non-READY backlog:\n%s", readFile(t, inbox))
	}

	// Two eligible rows: only the first (READY) mints; the second (INVERSION)
	// must not appear. Default empty allow-list ⇒ CONFIRM-REQUIRED status.
	plan2 := planFixture(t, dir,
		"| **first eligible** [class:exec] | — | first key | loom-author | queued | — |",
		"| **second eligible** [class:exec] | — | second key | loom-author | queued | — |",
	)
	inbox2 := writeFixture(t, dir, "inbox2.md", "")
	runPromote(t, nil, plan2, merged, inbox2)
	body := readFile(t, inbox2)
	if !strings.Contains(body, "promote-first-eligible") {
		t.Errorf("the READY (first eligible) row must mint:\n%s", body)
	}
	if strings.Contains(body, "second-eligible") {
		t.Errorf("the INVERSION (second eligible) row must NOT mint:\n%s", body)
	}
}

// TestPromoteNextFullKeyServes (case b, the M7 fix): the minted envelope carries
// the READY row's EXACT serves, and a sibling row whose serves is a substring of
// it is not conflated. Substring-but-not-equal must never match.
func TestPromoteNextFullKeyServes(t *testing.T) {
	dir := t.TempDir()
	merged := writeFixture(t, dir, "merged.txt", "")
	// Sibling 'lifecycle' is a substring of the READY row's 'agent lifecycle
	// continuity'. The READY row is first; its full serves must be minted exactly.
	plan := planFixture(t, dir,
		"| **ready row** [class:exec] | — | agent lifecycle continuity | loom-author | queued | — |",
		"| **sibling row** [class:exec] | — | lifecycle | loom-author | queued | — |",
	)
	inbox := writeFixture(t, dir, "inbox.md", "")
	runPromote(t, []string{"LOOM_AUTOPULL_CLASSES=exec"}, plan, merged, inbox)
	body := readFile(t, inbox)
	if !strings.Contains(body, "serves: agent lifecycle continuity") {
		t.Errorf("minted serves must be the READY row's full key, exact:\n%s", body)
	}
	// The sibling's bare 'lifecycle' must not be the minted serves line.
	if strings.Contains(body, "serves: lifecycle\n") {
		t.Errorf("substring sibling serves was used — M7 substring confusion:\n%s", body)
	}
}

// TestPromoteNextSelfSelectionFloor (case c): a row whose class is NOT on the
// pre-declared auto-pullable allow-list mints CONFIRM-REQUIRED (human tier), not
// QUEUED — distinct from the never-auto permission floor.
func TestPromoteNextSelfSelectionFloor(t *testing.T) {
	dir := t.TempDir()
	merged := writeFixture(t, dir, "merged.txt", "")
	plan := planFixture(t, dir,
		"| **ready row** [class:exec] | — | k | loom-author | queued | — |",
	)

	// Default: empty allow-list ⇒ CONFIRM-REQUIRED.
	inbox := writeFixture(t, dir, "inbox.md", "")
	runPromote(t, nil, plan, merged, inbox)
	if !strings.Contains(readFile(t, inbox), "status: CONFIRM-REQUIRED") {
		t.Errorf("off-allow-list row must mint CONFIRM-REQUIRED:\n%s", readFile(t, inbox))
	}
	// On the allow-list ⇒ QUEUED.
	inbox2 := writeFixture(t, dir, "inbox2.md", "")
	runPromote(t, []string{"LOOM_AUTOPULL_CLASSES=exec"}, plan, merged, inbox2)
	if !strings.Contains(readFile(t, inbox2), "status: QUEUED") {
		t.Errorf("allow-listed row must mint QUEUED:\n%s", readFile(t, inbox2))
	}
}

// TestPromoteNextIdempotent (case d): re-running over the same backlog + inbox
// does not double-mint the row.
func TestPromoteNextIdempotent(t *testing.T) {
	dir := t.TempDir()
	merged := writeFixture(t, dir, "merged.txt", "")
	plan := planFixture(t, dir,
		"| **ready row** [class:exec] | — | k | loom-author | queued | — |",
	)
	inbox := writeFixture(t, dir, "inbox.md", "")
	env := []string{"LOOM_AUTOPULL_CLASSES=exec"}
	runPromote(t, env, plan, merged, inbox)
	out := runPromote(t, env, plan, merged, inbox)
	if !strings.Contains(out, "idempotent") {
		t.Errorf("second run must report idempotent no-op:\n%s", out)
	}
	if n := strings.Count(readFile(t, inbox), "--- id: promote-"); n != 1 {
		t.Errorf("re-run double-minted: got %d envelopes, want 1", n)
	}
}

// TestPromoteNextSupersededNeverMints: a SUPERSEDED row (top of queue) is never
// READY, so it never mints — supersede-awareness inherited from readiness-decide.
func TestPromoteNextSupersededNeverMints(t *testing.T) {
	dir := t.TempDir()
	merged := writeFixture(t, dir, "merged.txt", "")
	plan := planFixture(t, dir,
		"| **dead row** [class:exec] [superseded-by:#9] | — | k | loom-author | queued | — |",
	)
	inbox := writeFixture(t, dir, "inbox.md", "")
	out := runPromote(t, []string{"LOOM_AUTOPULL_CLASSES=exec"}, plan, merged, inbox)
	if !strings.Contains(out, "nothing to promote") {
		t.Errorf("a superseded row must not promote:\n%s", out)
	}
	if strings.Contains(readFile(t, inbox), "--- id: promote-") {
		t.Errorf("superseded row minted an envelope:\n%s", readFile(t, inbox))
	}
}

// TestPromoteNextInjectionProof (case e): a row carrying a shell payload in its
// title/serves mints a structured envelope with NO shell execution (no file
// created), a sanitized [a-z0-9-] id slug, and a fixed body that never echoes the
// raw payload as an instruction (rehydration-poisoning mitigation, S2).
func TestPromoteNextInjectionProof(t *testing.T) {
	dir := t.TempDir()
	pwned := filepath.Join(dir, "PWNED")
	merged := writeFixture(t, dir, "merged.txt", "")
	// Payload in the title cell; no '|' (would break the column). The slug must
	// sanitize it away and no shell must run it.
	plan := planFixture(t, dir,
		fmt.Sprintf("| **evil row** [class:exec] x;touch ${IFS}%s `id` $(whoami) | — | k | loom-author | queued | — |", pwned),
	)
	inbox := writeFixture(t, dir, "inbox.md", "")
	runPromote(t, []string{"LOOM_AUTOPULL_CLASSES=exec"}, plan, merged, inbox)

	if _, err := os.Stat(pwned); !os.IsNotExist(err) {
		t.Errorf("INJECTION: %s exists — a row cell reached a shell (err=%v)", pwned, err)
	}
	body := readFile(t, inbox)
	// The id slug must be sanitized: only the literal `--- id: promote-` prefix
	// followed by [a-z0-9-].
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "--- id: promote-") {
			slug := strings.TrimPrefix(line, "--- id: promote-")
			if strings.TrimFunc(slug, func(r rune) bool {
				return r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
			}) != "" {
				t.Errorf("id slug not sanitized to [a-z0-9-]: %q", slug)
			}
		}
	}
	// No SHELL-METACHAR payload may survive anywhere in the minted envelope —
	// the id slug is sanitized to [a-z0-9-] and the body is a fixed template, so
	// none of the dangerous raw forms (command sub, ${IFS}, ;, the target path)
	// can appear. (Plain letters like "touch" inside a hyphenated slug are inert
	// data, not an instruction — only the metachar forms are the threat.)
	for _, bad := range []string{"${IFS}", "$(", "`", ";touch", pwned} {
		if strings.Contains(body, bad) {
			t.Errorf("raw payload form %q leaked into the minted envelope:\n%s", bad, body)
		}
	}
}
