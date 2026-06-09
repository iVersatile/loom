//go:build frcheck

package fr

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestFRRegistry is the FR-traceability check (ADR-0013), run via `make fr-verify`
// and the CI boundary job — NOT the per-commit gate (advisory by default, blocking
// at the merge boundary). It enforces both joints of spec -> FR -> test:
//
//   - FR -> test (blocking): every test an FR cites resolves to a real `func
//     Test...`; a dangling reference fails. "Passing" is transitive — the gate is
//     green, so an existing linked test is a passing one.
//   - spec -> FR (blocking): every cited spec file exists and its #anchor still
//     appears (loose, dashes->spaces, case-insensitive), catching FRs orphaned when
//     a spec clause is removed.
//
// Orphan tests (a test no FR references) are reported ADVISORY (never fatal).
// covers:/patterns: are checked as DECLARED (the suite resolves); confirming a
// suite truly exercises every verb/pattern is left to review.
func TestFRRegistry(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	frs := parseRegistry(t, filepath.Join(root, "docs", "FR-registry.yml"))
	if len(frs) == 0 {
		t.Fatal("parsed no FRs from docs/FR-registry.yml")
	}
	tests := scanTestFuncs(t, root) // pkg -> set(testName)

	referenced := map[string]bool{}
	for _, fr := range frs {
		if len(fr.testRefs) == 0 {
			t.Errorf("%s: no linked test (every FR needs >=1)", fr.id)
		}
		for _, ref := range fr.testRefs { // FR -> test
			referenced[ref] = true
			pkg, name := splitRef(ref)
			if !tests[pkg][name] {
				t.Errorf("%s: dangling test ref %q (no func %s in package %s)", fr.id, ref, name, pkg)
			}
		}
		for _, cite := range fr.sources { // spec -> FR
			file, anchor := resolveCite(root, cite)
			data, err := os.ReadFile(file)
			if err != nil {
				t.Errorf("%s: source %q -> missing file %s", fr.id, cite, file)
				continue
			}
			if anchor != "" {
				want := strings.ToLower(strings.ReplaceAll(anchor, "-", " "))
				if !strings.Contains(strings.ToLower(string(data)), want) {
					t.Errorf("%s: source %q -> anchor %q not found in %s", fr.id, cite, anchor, filepath.Base(file))
				}
			}
		}
	}

	var orphans []string // advisory
	for pkg, names := range tests {
		for name := range names {
			if ref := pkg + "." + name; !referenced[ref] {
				orphans = append(orphans, ref)
			}
		}
	}
	sort.Strings(orphans)
	t.Logf("advisory: %d FRs; %d test funcs unreferenced by any FR (orphans):\n  %s",
		len(frs), len(orphans), strings.Join(orphans, "\n  "))
}

type fr struct {
	id       string
	sources  []string
	testRefs []string // pkg.TestName
}

var (
	idRe   = regexp.MustCompile(`^\s*- id:\s*(\S+)`)
	srcRe  = regexp.MustCompile(`^\s*source:\s*(.+)$`)
	refRe  = regexp.MustCompile(`([a-z][a-z0-9]*)\.(Test[A-Za-z0-9_]+)`)
	funcRe = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)
)

// parseRegistry reads the requirements list line-by-line (skipping the header
// comments, which carry illustrative examples) and extracts each FR's id, source
// citations, and pkg.Test references.
func parseRegistry(t *testing.T, path string) []fr {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var frs []fr
	var cur *fr
	inReqs := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "requirements:" {
			inReqs = true
			continue
		}
		if !inReqs {
			continue
		}
		if m := idRe.FindStringSubmatch(line); m != nil {
			if cur != nil {
				frs = append(frs, *cur)
			}
			cur = &fr{id: m[1]}
			continue
		}
		if cur == nil {
			continue
		}
		if m := srcRe.FindStringSubmatch(line); m != nil {
			cur.sources = splitList(m[1])
			continue
		}
		if strings.HasPrefix(trimmed, "tests:") {
			for _, m := range refRe.FindAllStringSubmatch(line, -1) {
				cur.testRefs = append(cur.testRefs, m[1]+"."+m[2])
			}
		}
	}
	if cur != nil {
		frs = append(frs, *cur)
	}
	return frs
}

func scanTestFuncs(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		pkg := filepath.Base(filepath.Dir(p))
		for _, m := range funcRe.FindAllStringSubmatch(string(data), -1) {
			if out[pkg] == nil {
				out[pkg] = map[string]bool{}
			}
			out[pkg][m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func splitList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitRef(ref string) (pkg, name string) {
	if i := strings.IndexByte(ref, '.'); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return "", ref
}

// resolveCite maps a citation to a file path + anchor. A bare `*.md` resolves under
// docs/; anything with a slash (e.g. config/hooks/x) is repo-relative.
func resolveCite(root, cite string) (file, anchor string) {
	path := cite
	if i := strings.IndexByte(cite, '#'); i >= 0 {
		path, anchor = cite[:i], cite[i+1:]
	}
	if strings.HasSuffix(path, ".md") && !strings.Contains(path, "/") {
		path = filepath.Join("docs", path)
	}
	return filepath.Join(root, path), anchor
}
