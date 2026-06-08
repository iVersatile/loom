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

// TestSpecConformance asserts each verb's live --json top-level keys match the
// shape documented under its section in SPEC-verbs.md.
func TestSpecConformance(t *testing.T) {
	spec := specShapeKeys(t)

	cases := []struct {
		verb string
		args []string
	}{
		{"detect", []string{"detect", "--json"}},
		{"plan", []string{"plan", "--json"}},
		{"build", []string{"build", "--json"}},
		{"teardown", []string{"teardown", "stop", "--json"}},
	}

	for _, c := range cases {
		want, ok := spec[c.verb]
		if !ok {
			t.Errorf("%s: no documented --json shape found in %s", c.verb, specPath)
			continue
		}
		out, _, err := runCmd(t, c.args...)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", c.verb, err)
		}
		got := jsonTopKeys(t, out)
		if !slices.Equal(got, want) {
			t.Errorf("%s --json keys = %v, but SPEC-verbs.md documents %v", c.verb, got, want)
		}
	}
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
