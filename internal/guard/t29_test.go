package guard

import (
	"os/exec"
	"strings"
	"testing"
)

// T29 segment-aware guard-bash (envelope 046; design + binding amendments).
// The construction under test: per-segment pass (certain splits only) +
// whole-line legacy pass (deny floor never shrinks) + indirection
// conjunction — and the two LICENSED pattern re-anchorings, each pinned by a
// was-false-blocking regression below.

// runGuardBash returns (blocked, stderr).
func runGuardBash(t *testing.T, cmd string) (bool, string) {
	t.Helper()
	c := exec.Command("sh", absHook(t, "guard-bash"), cmd)
	c.Env = hermeticEnv()
	out, err := c.CombinedOutput()
	return err != nil, string(out)
}

// TestT29WasFalseBlockingSpecimens: the two live 2026-06-12 false blocks
// (046 amendment 2) must stop blocking.
func TestT29WasFalseBlockingSpecimens(t *testing.T) {
	for _, benign := range []string{
		// Specimen (b): recursive delete of a tmp dir — the old pattern
		// anchored on the slash after the flag, not the path root.
		"rm -rf /tmp/scratch-dir",
		"rm -rf /tmp/rpu-test && go test ./scripts/",
		// ...and its second firing: a draft QUOTING the first block.
		"echo 'guard-bash blocked: rm -rf /tmp/scratch-dir'",
		// Specimen (a): --force and main in UNRELATED chain segments must
		// not assemble a match (anchored-within-one-logical-command).
		"make build --force && git checkout main",
		"loom plan --force-color && git log main",
	} {
		if blocked, out := runGuardBash(t, benign); blocked {
			t.Errorf("was-false-blocking regression — must NOT block %q\n%s", benign, out)
		}
	}
}

// TestT29DenyFloorNeverShrinks: everything dangerous that blocked before
// still blocks — root deletes in every delimiter dress, home deletes,
// force-pushes (now in any flag order), and the H3/H4 bypass classes are
// covered by TestGuardBashBlocksBypassClasses unchanged.
func TestT29DenyFloorNeverShrinks(t *testing.T) {
	for _, danger := range []string{
		"rm -rf /",
		"rm -fr /",
		"rm -rf / --no-preserve-root",
		"rm -rf //",
		"rm -rf /*",
		"rm -rf /.",
		`sh -c "rm -rf /"`, // quote-adjacent root (uncertain parse -> whole line)
		"cd /tmp && rm -rf /",
		"rm -rf ~/anything",
		"git push --force",
		"git push -f",
		"git commit -m x && git push --force origin feat", // dangerous segment in a chain
		"git status # rm -rf /",                           // comment never stripped: uncertainty = one segment, conservative block
	} {
		if blocked, _ := runGuardBash(t, danger); !blocked {
			t.Errorf("deny floor shrank — must BLOCK %q", danger)
		}
	}
}

// TestT29SegmentAnchoredGainsFlagOrder: the re-anchored force-push pattern
// WIDENS coverage within a segment (reordered flags) — precision bought
// protection, not the reverse.
func TestT29SegmentAnchoredGainsFlagOrder(t *testing.T) {
	for _, danger := range []string{
		"git push origin main --force",
		"git push origin main -f",
	} {
		if blocked, _ := runGuardBash(t, danger); !blocked {
			t.Errorf("segment-anchored pattern must BLOCK reordered force-push %q", danger)
		}
	}
	// The anchored set must NOT leak to whole-line: pieces in different
	// segments stay unmatched (that is its entire anchor).
	if blocked, out := runGuardBash(t, "git push origin feat && tar -f x.tar -c ."); blocked {
		t.Errorf("anchored multi-token pattern matched across segments:\n%s", out)
	}
}

// TestT29IndirectionTaint: assignment + expansion assembling a force-push
// must still block (design point 3 / amendment 4) — in either token order —
// while plain expansion or unrelated assignments stay benign.
func TestT29IndirectionTaint(t *testing.T) {
	for _, danger := range []string{
		"FLAG=--force; git push $FLAG origin main",
		"FLAG=--force && git push origin main $FLAG",
	} {
		if blocked, _ := runGuardBash(t, danger); !blocked {
			t.Errorf("variable-indirection force-push must BLOCK %q", danger)
		}
	}
	for _, benign := range []string{
		"BRANCH=feat-x && git push origin $BRANCH", // assignment, no blocked token
		"git push origin $BRANCH",                  // expansion, no assignment
		"FMT=%h && git log --format=$FMT main",     // assignment + expansion, nothing armed
	} {
		if blocked, out := runGuardBash(t, benign); blocked {
			t.Errorf("indirection taint over-fired — must NOT block %q\n%s", benign, out)
		}
	}
}

// TestT29ConcatenationStaysWholeLine (amendment 4): a concatenation evasion
// (F=--for; F=$F"ce") carries quotes + expansion, so the splitter must
// refuse to split — whole-line semantics, the strictly-more-conservative
// posture. (Substring matching cannot see the assembled token; that residual
// is unchanged from the pre-T29 match set and accepted by the ruling.)
func TestT29ConcatenationStaysWholeLine(t *testing.T) {
	lib := absHook(t, "segment-split")
	probe := `. ` + shQuote(lib) + ` && if split_segments 'F=--for; F=$F"ce"; git push $F origin main' >/dev/null; then echo SPLIT; else echo UNSPLIT; fi`
	c := exec.Command("sh", "-c", probe)
	c.Env = hermeticEnv()
	out, err := c.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "UNSPLIT") {
		t.Errorf("concatenation/quoted line must be UNSPLIT (whole-line semantics), got %q err=%v", out, err)
	}
}

// TestT29SplitterUncertaintyClasses: each construct the splitter cannot
// parse with certainty must yield UNSPLIT (fail-closed by shape) — and a
// plain connector chain must split.
func TestT29SplitterUncertaintyClasses(t *testing.T) {
	lib := absHook(t, "segment-split")
	run := func(cmd string) string {
		t.Helper()
		probe := `. ` + shQuote(lib) + ` && if split_segments ` + shQuote(cmd) + ` >/dev/null; then echo SPLIT; else echo UNSPLIT; fi`
		c := exec.Command("sh", "-c", probe)
		c.Env = hermeticEnv()
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("splitter probe failed for %q: %v: %s", cmd, err, out)
		}
		return string(out)
	}

	for _, uncertain := range []string{
		`echo "a && b"`,        // double quotes
		`echo 'a; b'`,          // single quotes
		`echo $(date) && ls`,   // command substitution
		"echo `date`",          // backtick substitution
		`(cd /tmp && ls)`,      // subshell
		`go test ./... > out`,  // redirection
		`git status # comment`, // comment — no strip
		`git push origin $REF`, // expansion
	} {
		if !strings.Contains(run(uncertain), "UNSPLIT") {
			t.Errorf("splitter must be UNCERTAIN about %q", uncertain)
		}
	}
	if !strings.Contains(run("git add -A && git commit -m x; ls | wc -l"), "SPLIT") {
		t.Error("a plain connector chain must split")
	}
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
