package main

import (
	"os"
	"strings"
	"testing"
)

const planBase = `# PLAN
<!-- BEGIN TACTICAL QUEUE (agent-maintained — agents edit ONLY this fenced
     section; the phase roadmap below is human-owned) -->
## Tactical queue

| task | depends-on | serves | owner | status | PR |
| --- | --- | --- | --- | --- | --- |
| row-a | — | x | author | queued | — |
| row-b | — | x | author | queued | — |
| row-c | — | x | author | queued | — |
<!-- END TACTICAL QUEUE -->
roadmap stays
`

// TestRowUnionMerge proves the resolver's whole point (TEAM.md git
// discipline rule 4): one-side edits land from BOTH sides, additions and
// deletions union — where HEAD-wins would have silently reverted theirs'
// row-b edit and resurrected their deleted row-c.
func TestRowUnionMerge(t *testing.T) {
	ours := strings.Replace(planBase,
		"| row-a | — | x | author | queued | — |",
		"| row-a | — | x | author | done | #9 |", 1)
	ours = strings.Replace(ours,
		"| row-c | — | x | author | queued | — |",
		"| row-c | — | x | author | queued | — |\n| row-new | — | y | author | queued | — |", 1)
	theirs := strings.Replace(planBase,
		"| row-b | — | x | author | queued | — |",
		"| row-b | — | x | author | in review | #8 |", 1)
	theirs = strings.Replace(theirs, "| row-c | — | x | author | queued | — |\n", "", 1)

	got, code := resolve(planBase, ours, theirs, os.Stderr)
	if code != 0 {
		t.Fatalf("resolve exit %d, want 0", code)
	}
	for _, want := range []string{
		"| row-a | — | x | author | done | #9 |",      // ours' edit kept
		"| row-b | — | x | author | in review | #8 |", // theirs' edit kept
		"| row-new | — | y | author | queued | — |",   // ours' addition kept
		"roadmap stays", // outside-fence intact
	} {
		if !strings.Contains(got, want) {
			t.Errorf("resolved doc missing %q", want)
		}
	}
	if strings.Contains(got, "| row-c |") {
		t.Error("theirs deleted row-c; the union must not resurrect it")
	}
	if strings.Index(got, "| row-b |") > strings.Index(got, "| row-new |") {
		t.Error("ours-only row must slot after its ours-predecessor (row-c gone -> after row-b)")
	}
}

// TestTrueConflictExits2: both sides changing the same row differently is
// never guessed — exit 2, no output.
func TestTrueConflictExits2(t *testing.T) {
	ours := strings.Replace(planBase, "| row-a | — | x | author | queued | — |",
		"| row-a | — | x | author | done | #9 |", 1)
	theirs := strings.Replace(planBase, "| row-a | — | x | author | queued | — |",
		"| row-a | — | x | author | parked | — |", 1)

	if _, code := resolve(planBase, ours, theirs, os.Stderr); code != 2 {
		t.Fatalf("conflicting row edits: exit %d, want 2", code)
	}
}

// TestOutsideFenceDifferenceExits3: the human-owned roadmap is out of scope.
func TestOutsideFenceDifferenceExits3(t *testing.T) {
	theirs := strings.Replace(planBase, "roadmap stays", "roadmap moved", 1)
	if _, code := resolve(planBase, planBase, theirs, os.Stderr); code != 3 {
		t.Fatalf("outside-fence difference: exit %d, want 3", code)
	}
}

// TestDeletedVsChangedConflicts: a row deleted on one side but edited on the
// other is a decision, not a merge.
func TestDeletedVsChangedConflicts(t *testing.T) {
	ours := strings.Replace(planBase, "| row-c | — | x | author | queued | — |\n", "", 1)
	theirs := strings.Replace(planBase, "| row-c | — | x | author | queued | — |",
		"| row-c | — | x | author | done | #7 |", 1)
	if _, code := resolve(planBase, ours, theirs, os.Stderr); code != 2 {
		t.Fatalf("delete-vs-change: exit %d, want 2", code)
	}
}

// TestIdempotentNoChanges: identical inputs resolve to the input.
func TestIdempotentNoChanges(t *testing.T) {
	got, code := resolve(planBase, planBase, planBase, os.Stderr)
	if code != 0 || got != planBase {
		t.Fatalf("no-op resolve must return the input verbatim (exit %d)", code)
	}
}
