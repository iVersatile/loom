package cli

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// specPath is the contract document this test enforces. Treating the spec as a
// contract artifact (not just prose): the documented --json shapes are extracted
// here and compared against the live CLI output, so any drift between
// SPEC-verbs.md and the engine fails the gate (RULES §2, ADR-0006).
const specPath = "../../docs/SPEC-verbs.md"

// fixturePlaybook is a fully-formed project playbook (with a config tree) used by
// verbs that require one. Reused from the playbook package's testdata.
const fixturePlaybook = "../playbook/testdata/proj/loom.yml"

// TestSpecConformance asserts each verb's live --json top-level keys match the
// shape documented under its section in SPEC-verbs.md. It checks SHAPE only:
// every verb emits its result before any error/exit-code, so a verb that exits
// non-zero (plan drift) or fails its container step (build without docker) still
// proves its contract. build mutates, so it runs against a sandboxed temp copy.
func TestSpecConformance(t *testing.T) {
	// Shape-only, and it MUST stay hermetic to host tooling: this runs the real
	// `build`/`teardown`, which on a host that HAS docker would provision a real
	// container (compiling gopls) instead of failing fast — making the unit gate
	// slow and, under -timeout, a hard timeout (it hit 120s in CI; local sandboxes
	// have no docker so it looked fast). Point PATH at an empty dir so docker and
	// the prober's tool lookups fail immediately; the result JSON is still emitted,
	// which is all this test asserts. (LL-008)
	t.Setenv("PATH", t.TempDir())
	// start's required key is supplied via the environment (the os.Getenv
	// pass-through path), so the case needs no NAME=VALUE flag literal.
	t.Setenv("ANTHROPIC_API_KEY", "x")
	spec := specShapeKeys(t)

	// build, start, and teardown mutate / open the action log — sandbox them in
	// temp copies. start orchestrates build, so it gets its own copy; with an
	// empty PATH its container step fails fast, but the full --json result is
	// still emitted (ready:false), which is all this shape check asserts.
	build := tempCopy(t, "../playbook/testdata/proj") + "/loom.yml"
	start := tempCopy(t, "../playbook/testdata/proj") + "/loom.yml"
	teardown := tempCopy(t, "../playbook/testdata/proj") + "/loom.yml"
	cases := []struct {
		verb string
		args []string
	}{
		{"detect", []string{"detect", "--json", "-f", fixturePlaybook}},
		{"plan", []string{"plan", "--json", "-f", fixturePlaybook}},
		{"build", []string{"build", "--json", "-f", build}},
		{"start", []string{"start", "--non-interactive", "--json", "--stack", "go", "-f", start}},
		{"teardown", []string{"teardown", "stop", "--json", "--yes", "-f", teardown}},
	}

	for _, c := range cases {
		want, ok := spec[c.verb]
		if !ok {
			t.Errorf("%s: no documented --json shape found in %s", c.verb, specPath)
			continue
		}
		// Errors/exit codes are expected for some verbs in this environment; the
		// result JSON is still emitted, and the shape is what we assert.
		out, _, _ := runCmd(t, c.args...)
		got := jsonTopKeys(t, out)
		if !slices.Equal(got, want) {
			t.Errorf("%s --json keys = %v, but SPEC-verbs.md documents %v", c.verb, got, want)
		}
	}
}

// tempCopy copies a directory tree into a fresh temp dir so a verb that mutates
// (build) never touches the committed testdata.
func tempCopy(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
	return dst
}

// specShapeKeys parses SPEC-verbs.md and returns, per verb section, the sorted
// top-level keys of the first ```json shape block under that section.
func specShapeKeys(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	out := map[string][]string{}
	var verb string
	var inJSON bool
	var buf []string
	for _, ln := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(ln, "## "):
			verb = strings.Fields(strings.TrimPrefix(ln, "## "))[0]
		case strings.HasPrefix(strings.TrimSpace(ln), "```json"):
			inJSON, buf = true, nil
		case inJSON && strings.HasPrefix(strings.TrimSpace(ln), "```"):
			inJSON = false
			if _, seen := out[verb]; !seen && verb != "" {
				if keys := parseTopKeys(strings.Join(buf, "\n")); keys != nil {
					out[verb] = keys
				}
			}
		case inJSON:
			buf = append(buf, stripLineComment(ln))
		}
	}
	return out
}

// stripLineComment removes a // comment so the spec's annotated shape blocks
// become valid JSON. Handles both full-line comments and trailing " // ..."
// (no shape value contains "//", so this is safe for SPEC-verbs.md).
func stripLineComment(ln string) string {
	if strings.HasPrefix(strings.TrimSpace(ln), "//") {
		return ""
	}
	if i := strings.Index(ln, " //"); i >= 0 {
		return ln[:i]
	}
	return ln
}

func parseTopKeys(s string) []string {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(s), &m) != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func jsonTopKeys(t *testing.T, s string) []string {
	t.Helper()
	keys := parseTopKeys(s)
	if keys == nil {
		t.Fatalf("output is not valid JSON object:\n%s", s)
	}
	return keys
}
