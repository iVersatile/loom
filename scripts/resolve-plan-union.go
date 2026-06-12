// resolve-plan-union — three-way ROW-UNION merge for the PLAN tactical queue.
//
// # name   : resolve-plan-union — queue-table merge resolver (TEAM.md git discipline rule 4)
// # Origin : envelope 041 / human-blessed draft 028 (2026-06-12) — promoted from
// #          the advisor's /tmp/resolve-plan-union.py seed (8/8 rows live).
// #          Transcribed python -> Go at promotion: loom-dev ships no python, and
// #          rule 4 is mechanism only if EVERY seat can run it (`go run`).
// #          HEAD-wins resolution is WRONG for the queue table: it silently
// #          reverts rows the branch never edited (caught live 2026-06-12).
// # Reuse  : recurring (verb-candidate: /replan integration or a git merge driver)
// # Runs on: all three (Mac host / devenv / loom-dev)
//
// Usage:
//
//	go run ./scripts [-o OUT] BASE OURS THEIRS
//
// BASE/OURS/THEIRS are three versions of docs/PLAN.md (merge-base, our branch,
// origin/main). The resolved document goes to OUT (default: stdout). Typical
// invocation during a conflicted merge/rebase:
//
//	git show :1:docs/PLAN.md > /tmp/base.md
//	git show :2:docs/PLAN.md > /tmp/ours.md
//	git show :3:docs/PLAN.md > /tmp/theirs.md
//	go run ./scripts -o docs/PLAN.md /tmp/base.md /tmp/ours.md /tmp/theirs.md
//
// (`go run` collapses any non-zero exit to 1, printing `exit status N` —
// if you script on the 2-vs-3 distinction, `go build -o` first.)
//
// Resolution, per row (keyed by the first cell, the task name):
//
//	changed on one side only                    -> take the changed side
//	changed identically on both                 -> take it
//	changed differently on both                 -> TRUE CONFLICT: report, exit 2 (never guess)
//	added on either side                        -> keep (the union)
//	deleted on one side, untouched on the other -> delete
//	deleted on one side, changed on the other   -> TRUE CONFLICT, exit 2
//
// Row spine follows THEIRS (origin/main) order; ours-only rows slot in after
// their predecessor in OURS, else append. Content outside the fenced queue
// section is NOT this tool's domain: any ours/theirs difference there is a
// manual resolve, exit 3.
//
// HARD RULE (TEAM.md "Shared-tree git discipline" rule 4): ALWAYS diff the
// resolved PLAN against origin/main before pushing:
//
//	git diff origin/main -- docs/PLAN.md
//
// The tool prints this reminder on every successful resolve.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

const (
	fenceBegin = "<!-- BEGIN TACTICAL QUEUE"
	fenceEnd   = "<!-- END TACTICAL QUEUE"
)

func main() {
	out := flag.String("o", "", "write resolved doc here (default stdout)")
	flag.Parse()
	if flag.NArg() != 3 {
		sayf(os.Stderr, "usage: resolve-plan-union [-o OUT] BASE OURS THEIRS\n")
		os.Exit(64)
	}

	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			fatal(1, "read %s: %v", p, err)
		}
		return string(b)
	}
	text, code := resolve(read(flag.Arg(0)), read(flag.Arg(1)), read(flag.Arg(2)), os.Stderr)
	if code != 0 {
		os.Exit(code)
	}
	if *out != "" {
		if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
			fatal(1, "write %s: %v", *out, err)
		}
	} else if _, err := os.Stdout.WriteString(text); err != nil {
		fatal(1, "write stdout: %v", err)
	}
}

func fatal(code int, format string, a ...any) {
	sayf(os.Stderr, format+"\n", a...)
	os.Exit(code)
}

// sayf is Fprintf with the error deliberately dropped: diagnostics are
// best-effort (errcheck-clean without ceremony at every call site).
func sayf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

// doc is one version of PLAN.md split at the queue fences.
type doc struct {
	head  string   // through the BEGIN fence's closing -->
	queue []string // lines between the fences
	tail  string   // from the END fence on
}

type queueTable struct {
	scaffold []string          // prose + header + |---| separator, in order
	rows     map[string]string // task-name key -> full row line
	order    []string
}

// resolve runs the row-union merge and returns the resolved document and an
// exit code (0 ok; 2 true conflicts; 3 out-of-scope difference). Diagnostics
// and the hard-rule reminder go to diag.
func resolve(base, ours, theirs string, diag io.Writer) (string, int) {
	db, ok1 := splitDoc(base)
	do, ok2 := splitDoc(ours)
	dt, ok3 := splitDoc(theirs)
	if !ok1 || !ok2 || !ok3 {
		sayf(diag, "error: tactical-queue fences not found or malformed in an input\n")
		return "", 3
	}
	if do.head != dt.head || do.tail != dt.tail {
		sayf(diag, "error: ours/theirs differ OUTSIDE the fenced queue section — that part is human-owned; resolve it manually\n")
		return "", 3
	}

	tb, err := parseRows(db.queue)
	if err == nil {
		var to, tt *queueTable
		if to, err = parseRows(do.queue); err == nil {
			tt, err = parseRows(dt.queue)
			if err == nil {
				return merge(tb, to, tt, dt, diag)
			}
		}
	}
	sayf(diag, "error: %v\n", err)
	return "", 3
}

func merge(b, o, t *queueTable, dt doc, diag io.Writer) (string, int) {
	if !slices.Equal(o.scaffold, t.scaffold) {
		sayf(diag, "error: queue scaffold (header/prose) differs between ours and theirs — resolve manually\n")
		return "", 3
	}

	resolved := map[string]string{}
	var conflicts []string
	for _, key := range union(t.order, o.order) {
		bRow, inB := b.rows[key]
		oRow, inO := o.rows[key]
		tRow, inT := t.rows[key]
		var keep string
		var kept bool
		switch {
		case inO == inT && oRow == tRow: // identical, incl. both deleted
			keep, kept = oRow, inO
		case inO == inB && oRow == bRow: // only theirs changed, incl. theirs deleted
			keep, kept = tRow, inT
		case inT == inB && tRow == bRow: // only ours changed, incl. ours deleted
			keep, kept = oRow, inO
		default:
			conflicts = append(conflicts, key)
			sayf(diag, "\n## row: %s\n  ours:   %s\n  theirs: %s\n",
				key, orDeleted(oRow, inO), orDeleted(tRow, inT))
			continue
		}
		if kept {
			resolved[key] = keep
		}
	}
	if len(conflicts) > 0 {
		sayf(diag, "\nTRUE CONFLICTS — both sides changed %d row(s) above; resolve by hand\n", len(conflicts))
		return "", 2
	}

	// Spine: theirs order; ours-only rows slot after their ours-predecessor.
	var spine []string
	for _, k := range t.order {
		if _, ok := resolved[k]; ok {
			spine = append(spine, k)
		}
	}
	for i, k := range o.order {
		if _, ok := resolved[k]; !ok || slices.Contains(spine, k) {
			continue
		}
		at := len(spine)
		for j := i - 1; j >= 0; j-- {
			if p := slices.Index(spine, o.order[j]); p >= 0 {
				at = p + 1
				break
			}
		}
		spine = slices.Insert(spine, at, k)
	}

	var q strings.Builder
	emitted := false
	for _, ln := range o.scaffold {
		q.WriteString(ln)
		if !emitted && isSeparatorRow(ln) {
			for _, k := range spine {
				q.WriteString(resolved[k])
			}
			emitted = true
		}
	}
	if !emitted {
		sayf(diag, "error: queue table header/separator not found\n")
		return "", 3
	}

	sayf(diag, "resolved: %d rows (%d ours / %d theirs / %d base)\n", len(spine), len(o.rows), len(t.rows), len(b.rows))
	sayf(diag, "HARD RULE: diff the resolved PLAN against origin/main BEFORE pushing:\n    git diff origin/main -- docs/PLAN.md\n")
	return dt.head + q.String() + dt.tail, 0
}

func splitDoc(text string) (doc, bool) {
	lines := strings.SplitAfter(text, "\n")
	begin, end := -1, -1
	for i, ln := range lines {
		switch {
		case strings.HasPrefix(ln, fenceBegin):
			begin = i
		case strings.HasPrefix(ln, fenceEnd):
			end = i
		}
	}
	if begin < 0 || end <= begin {
		return doc{}, false
	}
	j := begin
	for j <= end && !strings.Contains(lines[j], "-->") {
		j++
	}
	return doc{
		head:  strings.Join(lines[:j+1], ""),
		queue: lines[j+1 : end],
		tail:  strings.Join(lines[end:], ""),
	}, true
}

func parseRows(queue []string) (*queueTable, error) {
	t := &queueTable{rows: map[string]string{}}
	for _, ln := range queue {
		if !strings.HasPrefix(ln, "|") {
			t.scaffold = append(t.scaffold, ln)
			continue
		}
		key := rowKey(ln)
		if key == "task" || isSeparatorRow(ln) {
			t.scaffold = append(t.scaffold, ln)
			continue
		}
		if _, dup := t.rows[key]; dup {
			return nil, fmt.Errorf("duplicate queue row key: %q", key)
		}
		t.rows[key] = ln
		t.order = append(t.order, key)
	}
	return t, nil
}

func rowKey(ln string) string {
	cells := strings.Split(strings.Trim(strings.TrimSpace(ln), "|"), "|")
	return strings.TrimSpace(cells[0])
}

func isSeparatorRow(ln string) bool {
	s := strings.TrimSpace(ln)
	if !strings.HasPrefix(s, "|") {
		return false
	}
	return strings.Trim(s, "|-: ") == ""
}

func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, k := range b {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

func orDeleted(row string, present bool) string {
	if !present {
		return "<deleted>"
	}
	return strings.TrimSpace(row)
}
