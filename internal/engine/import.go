package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
	"sigs.k8s.io/yaml"
)

// importedPlaybookName is the DISTINCT draft file `loom import` writes — never
// the canonical loom.yml, so an imported devcontainer can be reviewed then
// committed without clobbering an authored playbook (docs/SPEC-verbs.md#import).
const importedPlaybookName = "loom.imported.yml"

// ImportMapped is the subset of devcontainer fields that have a playbook home:
// ports and env NAMES (values stay in .env/secret store — the names-only model).
type ImportMapped struct {
	Ports []int    `json:"ports"`
	Env   []string `json:"env"`
}

// ImportResult is the documented --json shape (docs/SPEC-verbs.md#import):
// source, mapped:{ports,env}, reported, deferred, draft.
type ImportResult struct {
	Source   string            `json:"source"`
	Mapped   ImportMapped      `json:"mapped"`
	Reported map[string]string `json:"reported"` // e.g. {"image": "..."}; empty map if nothing
	Deferred []string          `json:"deferred"` // e.g. ["features","commands"]
	Draft    string            `json:"draft"`
}

// Human renders the operator-facing one-liner.
func (r ImportResult) Human() string {
	var b strings.Builder
	fmt.Fprintf(&b, "import: %d ports, %d env from %s -> %s", len(r.Mapped.Ports), len(r.Mapped.Env), r.Source, r.Draft)

	var notes []string
	if len(r.Reported) > 0 {
		keys := make([]string, 0, len(r.Reported))
		for k := range r.Reported {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		notes = append(notes, "reported: "+strings.Join(keys, ", "))
	}
	if len(r.Deferred) > 0 {
		notes = append(notes, "deferred: "+strings.Join(r.Deferred, ", "))
	}
	if len(notes) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(notes, "; "))
	}
	return b.String()
}

// devcontainer is the subset of devcontainer.json fields Stage-1 import reads.
// Polymorphic fields (int|string|array) decode as json.RawMessage and are
// interpreted by hand.
type devcontainer struct {
	Name         string                     `json:"name"`
	Image        string                     `json:"image"`
	ForwardPorts []json.RawMessage          `json:"forwardPorts"`
	AppPort      json.RawMessage            `json:"appPort"`
	ContainerEnv map[string]string          `json:"containerEnv"`
	RemoteEnv    map[string]string          `json:"remoteEnv"`
	Features     map[string]json.RawMessage `json:"features"`

	PostCreateCommand    json.RawMessage `json:"postCreateCommand"`
	PostStartCommand     json.RawMessage `json:"postStartCommand"`
	PostAttachCommand    json.RawMessage `json:"postAttachCommand"`
	OnCreateCommand      json.RawMessage `json:"onCreateCommand"`
	UpdateContentCommand json.RawMessage `json:"updateContentCommand"`
}

// Import ingests a devcontainer.json into a reviewable DRAFT project-tier Loom
// playbook (docs/SPEC-verbs.md#import, ADR-0003 "import and enrich, never
// degrade to"). It reads a FILE and writes a draft; it never mutates the
// environment beyond writing that distinct draft file.
func Import(srcPath string) (ImportResult, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("import: read %s: %w", srcPath, err)
	}

	var dc devcontainer
	if err := json.Unmarshal(stripJSONC(raw), &dc); err != nil {
		return ImportResult{}, fmt.Errorf("import: parse %s as devcontainer JSON(C): %w", srcPath, err)
	}

	ports := collectPorts(dc)
	env := collectEnvNames(dc)

	name := sanitizeName(dc.Name)
	if name == "" {
		// Project-root basename (sanitized): the dir the draft is written into.
		name = sanitizeName(filepath.Base(draftDir(srcPath)))
	}
	if name == "" {
		name = "imported-project"
	}

	pb := playbook.Playbook{
		Loom:  playbook.SchemaVersion,
		Tier:  playbook.TierProject,
		Name:  name,
		Ports: ports,
		Env:   env,
	}
	if err := pb.Validate(); err != nil {
		return ImportResult{}, fmt.Errorf("import: draft playbook invalid: %w", err)
	}

	// REPORTED: the devcontainer image is surfaced, never mapped to the playbook
	// (base_image is an engine-level floor, ADR-0012; loom enriches, ADR-0003).
	reported := map[string]string{}
	if dc.Image != "" {
		reported["image"] = dc.Image
	}

	// DEFERRED: fields with no schema home yet — reported, never silently dropped.
	// Non-nil so the --json `deferred` is [] not null when empty (AI-first shape).
	deferred := []string{}
	if len(dc.Features) > 0 {
		deferred = append(deferred, "features")
	}
	if hasCommand(dc) {
		deferred = append(deferred, "commands")
	}

	dir := draftDir(srcPath)
	draft := filepath.Join(dir, importedPlaybookName)
	// DEFENSIVE: never clobber the canonical playbook (can't happen with the
	// distinct name, but guard anyway).
	if filepath.Base(draft) == "loom.yml" {
		return ImportResult{}, fmt.Errorf("import: refusing to write draft over canonical playbook %q", draft)
	}

	data, err := yaml.Marshal(&pb)
	if err != nil {
		return ImportResult{}, fmt.Errorf("import: marshal draft: %w", err)
	}
	if err := os.WriteFile(draft, data, 0o644); err != nil {
		return ImportResult{}, fmt.Errorf("import: write draft %s: %w", draft, err)
	}

	return ImportResult{
		Source:   srcPath,
		Mapped:   ImportMapped{Ports: ports, Env: env},
		Reported: reported,
		Deferred: deferred,
		Draft:    draft,
	}, nil
}

// draftDir returns the directory the draft is written into: the parent of a
// .devcontainer/ dir (so the draft lands in the project root, not inside
// .devcontainer/), else the source file's own directory.
func draftDir(srcPath string) string {
	dir := filepath.Dir(srcPath)
	if filepath.Base(dir) == ".devcontainer" {
		dir = filepath.Dir(dir)
	}
	return dir
}

// collectPorts gathers ports from forwardPorts (array) + appPort (single or
// array), deduped and sorted ascending.
func collectPorts(dc devcontainer) []int {
	set := map[int]struct{}{}
	for _, e := range dc.ForwardPorts {
		if p, ok := portFromRaw(e); ok {
			set[p] = struct{}{}
		}
	}
	for _, p := range portsFromAppPort(dc.AppPort) {
		set[p] = struct{}{}
	}
	out := make([]int, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// portsFromAppPort interprets appPort, which may be a single int/string or an
// array of them.
func portsFromAppPort(raw json.RawMessage) []int {
	if len(raw) == 0 {
		return nil
	}
	// Try an array first.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		var out []int
		for _, e := range arr {
			if p, ok := portFromRaw(e); ok {
				out = append(out, p)
			}
		}
		return out
	}
	if p, ok := portFromRaw(raw); ok {
		return []int{p}
	}
	return nil
}

// portFromRaw extracts a port from a JSON int or string token. A "host:port"
// string yields the segment after the LAST ':' if numeric; a bare numeric
// string yields its int; anything else is skipped.
func portFromRaw(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if p, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return p, true
	}
	return 0, false
}

// collectEnvNames returns the union of containerEnv + remoteEnv KEYS (names
// only — values are intentionally dropped), deduped and sorted.
func collectEnvNames(dc devcontainer) []string {
	set := map[string]struct{}{}
	for k := range dc.ContainerEnv {
		set[k] = struct{}{}
	}
	for k := range dc.RemoteEnv {
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// hasCommand reports whether ANY devcontainer command field is present.
func hasCommand(dc devcontainer) bool {
	return len(dc.PostCreateCommand) > 0 ||
		len(dc.PostStartCommand) > 0 ||
		len(dc.PostAttachCommand) > 0 ||
		len(dc.OnCreateCommand) > 0 ||
		len(dc.UpdateContentCommand) > 0
}

// sanitizeName slugifies an imported devcontainer name into a docker-safe,
// playbook-valid token: lowercase, runs of disallowed chars collapse to a single
// '-', trimmed of leading/trailing separators. A devcontainer `name` ("My App",
// "org/repo") otherwise flows verbatim into the playbook `name:` — which becomes
// the container name — and would break docker naming. Returns "" if nothing
// usable remains (the caller then falls back).
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-._")
}

// stripJSONC converts JSONC (devcontainer.json's format) to strict JSON,
// preserving string contents verbatim. A single state-machine pass:
//   - inside a string, bytes are copied verbatim (a '//', '/*' or ',' there is
//     NOT a comment/separator); a backslash escapes the next byte; an unescaped
//     '"' ends the string.
//   - outside a string, '//' starts a line comment (skip to before the '\n'),
//     '/*' starts a block comment (skip through the closing '*/'), and a ','
//     whose next significant char (past whitespace + comments) is '}' or ']' is
//     dropped (trailing-comma removal).
//
// The footgun this guards: a //, /* or , INSIDE a string value (URLs, "a,b")
// must survive untouched.
func stripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inString := false
	escaped := false

	for i := 0; i < len(b); i++ {
		c := b[i]

		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		// Outside a string.
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			// Line comment: skip to (not including) the next newline.
			i += 2
			for i < len(b) && b[i] != '\n' {
				i++
			}
			i-- // let the loop's i++ land on the '\n' (or end)
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			// Block comment: skip through the closing '*/'.
			i += 2
			for i+1 < len(b) && (b[i] != '*' || b[i+1] != '/') {
				i++
			}
			i++ // consume the '*' ; loop's i++ consumes the '/'
		case c == ',':
			// Trailing comma: drop it if the next significant char (skipping
			// whitespace AND comments) is '}' or ']'.
			if j := nextSignificant(b, i+1); j < len(b) && (b[j] == '}' || b[j] == ']') {
				continue // drop the comma
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// nextSignificant returns the index of the next non-whitespace, non-comment
// byte at or after i, or len(b) if none. It does NOT enter strings (it only
// scans the gap between a ',' and a possible closing bracket, which contains no
// string), but it does skip line/block comments so `[1, /*x*/ ]` is handled.
func nextSignificant(b []byte, i int) int {
	for i < len(b) {
		c := b[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			i += 2
			for i < len(b) && b[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && (b[i] != '*' || b[i+1] != '/') {
				i++
			}
			i += 2
		default:
			return i
		}
	}
	return i
}
