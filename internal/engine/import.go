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
	Tools []string `json:"tools"`
}

// featureToTool maps a known official devcontainer feature name (the segment after
// ".../features/") to loom tool name(s). Unlisted official features and custom refs
// are REPORTED (unmapped_features), never guessed.
var featureToTool = map[string][]string{
	"go": {"go"}, "node": {"node"}, "python": {"python"}, "rust": {"rust"},
	"ruby": {"ruby"}, "java": {"java"}, "dotnet": {"dotnet"}, "php": {"php"},
	"git": {"git"}, "github-cli": {"gh"}, "aws-cli": {"awscli"}, "azure-cli": {"az"},
	"terraform": {"terraform"}, "kubectl-helm-minikube": {"kubectl", "helm"},
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
	fmt.Fprintf(&b, "import: %d ports, %d env, %d tools from %s -> %s", len(r.Mapped.Ports), len(r.Mapped.Env), len(r.Mapped.Tools), r.Source, r.Draft)

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

	// Lifecycle commands (devcontainer.json). Each is string | array | object;
	// they are CAPTURED into reported.commands for HUMAN REVIEW and NEVER executed
	// by loom (FR-IMPORT-005, ADR-0005 worst-thing test). Declared in devcontainer
	// lifecycle order so the captured list reads in execution order for the human.
	InitializeCommand    json.RawMessage `json:"initializeCommand"`
	OnCreateCommand      json.RawMessage `json:"onCreateCommand"`
	UpdateContentCommand json.RawMessage `json:"updateContentCommand"`
	PostCreateCommand    json.RawMessage `json:"postCreateCommand"`
	PostStartCommand     json.RawMessage `json:"postStartCommand"`
	PostAttachCommand    json.RawMessage `json:"postAttachCommand"`
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
	tools, unmappedFeatures := collectTools(dc)

	name := sanitizeName(dc.Name)
	if name == "" {
		// Project-root basename (sanitized): the dir the draft is written into.
		name = sanitizeName(filepath.Base(draftDir(srcPath)))
	}
	if name == "" {
		name = "imported-project"
	}

	// REPORTED commands: the devcontainer LIFECYCLE commands captured verbatim into
	// the DRAFT's reported.commands for human review — NEVER mapped to an executable
	// field and NEVER run (FR-IMPORT-005, ADR-0005 worst-thing test). Auto-running a
	// command from an imported (untrusted) devcontainer is a code-execution surface
	// the guardrails must not open; this is the "image is REPORTED, not mapped"
	// precedent applied to commands (ADR-0003 import-and-enrich-never-degrade-to).
	commands := collectCommands(dc)

	pb := playbook.Playbook{
		Loom:  playbook.SchemaVersion,
		Tier:  playbook.TierProject,
		Name:  name,
		Ports: ports,
		Env:   env,
		Tools: tools,
	}
	// The captured commands live in the draft's reported: section (declared schema,
	// never executed). They are review DATA, not desired-state — Validate/Merge/build
	// ignore them; they exist so the import is lossless and the draft round-trips.
	if len(commands) > 0 {
		pb.Reported = &playbook.Reported{Commands: commands}
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
	// REPORTED: unrecognized/custom features are surfaced (for the human / the
	// import-enrich skill), never guessed into tools.
	if len(unmappedFeatures) > 0 {
		reported["unmapped_features"] = strings.Join(unmappedFeatures, ", ")
	}
	// REPORTED: lifecycle commands — surfaced in --json as the lifecycle hook names
	// captured (the bodies live in the draft's reported.commands). Commands are NO
	// LONGER deferred: they have a home (the non-executable reported section), so the
	// import is lossless without ever becoming auto-runnable.
	if len(commands) > 0 {
		hooks := make([]string, len(commands))
		for i, c := range commands {
			hooks[i] = c.Hook
		}
		reported["commands"] = strings.Join(hooks, ", ")
	}

	// DEFERRED: fields with no home at all — reported, never silently dropped.
	// Non-nil so the --json `deferred` is [] not null when empty (AI-first shape).
	// `features` map to tools / reported; `commands` are now REPORTED (above), not
	// deferred — nothing remains in the deferred set for Stage-1 import.
	deferred := []string{}

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

	// Non-nil slices so the --json `mapped` sub-keys are [] not null when empty.
	if tools == nil {
		tools = []string{}
	}
	return ImportResult{
		Source:   srcPath,
		Mapped:   ImportMapped{Ports: ports, Env: env, Tools: tools},
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

// collectTools maps RECOGNIZED official devcontainer features to loom tool intents
// (name@version, or the bare name when no clean version), and REPORTS the rest:
// unrecognized/custom feature refs are returned as `unmapped` rather than guessed.
// The tool version is the feature's `version` OPTION (the options object value),
// NOT the ref tag (which is the feature's own version). Both returns are deduped +
// sorted; both default to nil (the caller materializes [] when needed).
func collectTools(dc devcontainer) (tools []string, unmapped []string) {
	toolSet := map[string]struct{}{}
	unmappedSet := map[string]struct{}{}
	for ref, optsRaw := range dc.Features {
		name, ok := featureName(ref)
		if !ok {
			unmappedSet[ref] = struct{}{}
			continue
		}
		mapped, known := featureToTool[name]
		if !known {
			unmappedSet[ref] = struct{}{}
			continue
		}
		ver := featureVersion(optsRaw)
		for _, tool := range mapped {
			// A multi-tool feature (e.g. kubectl-helm-minikube) has ONE `version`
			// option that pins only its primary tool, not the siblings — so attach
			// the version only when the feature maps to a single tool; otherwise
			// emit bare names rather than a wrong pin (helm@<kubectl-version>).
			if ver != "" && len(mapped) == 1 {
				toolSet[tool+"@"+ver] = struct{}{}
			} else {
				toolSet[tool] = struct{}{}
			}
		}
	}
	for t := range toolSet {
		tools = append(tools, t)
	}
	for u := range unmappedSet {
		unmapped = append(unmapped, u)
	}
	sort.Strings(tools)
	sort.Strings(unmapped)
	return tools, unmapped
}

// featureName recognizes an OFFICIAL devcontainers/features ref and returns its
// feature name (the segment after "features/", tag stripped). It requires the EXACT
// shape `<host>/devcontainers/features/<name>[:tag]` — a host-anchored boundary
// match, not a substring — so a CUSTOM ref that merely contains the official path
// (e.g. `ghcr.io/me/devcontainers/features/go`, `x-devcontainers/features/go`) is
// REPORTED, never wrongly guessed into the official tool. Examples:
//   - ghcr.io/devcontainers/features/go:1               -> ("go", true)
//   - mcr.microsoft.com/devcontainers/features/node:1   -> ("node", true)
//   - ghcr.io/acme/custom:2                             -> ("", false)
//   - ghcr.io/me/devcontainers/features/go:1            -> ("", false)  // 5 segments
func featureName(ref string) (string, bool) {
	// Exactly: host / "devcontainers" / "features" / name[:tag]
	segs := strings.Split(ref, "/")
	if len(segs) != 4 || segs[1] != "devcontainers" || segs[2] != "features" {
		return "", false
	}
	rest := segs[3]
	// Drop any ":tag" — the feature's own version, irrelevant to the tool version.
	if j := strings.IndexByte(rest, ':'); j >= 0 {
		rest = rest[:j]
	}
	if rest == "" {
		return "", false
	}
	return rest, true
}

// featureVersion reads the feature's `version` OPTION from its options object and
// returns a clean version token (reusing cleanVersion), or "" when the option is
// absent / not a clean token — so the caller emits the bare tool name.
func featureVersion(optsRaw json.RawMessage) string {
	if len(optsRaw) == 0 {
		return ""
	}
	var v struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(optsRaw, &v); err != nil {
		return ""
	}
	return cleanVersion(v.Version)
}

// collectCommands captures the present devcontainer LIFECYCLE commands into
// []playbook.ReportedCommand — verbatim, in devcontainer lifecycle order, for
// HUMAN REVIEW. It does NOT execute them and the result lands ONLY in the draft's
// non-executable reported.commands (FR-IMPORT-005, ADR-0005 worst-thing test). A
// hook whose value is present but yields no command strings is still captured (with
// an empty Run) so the human sees the hook was declared — lossless, never dropped.
func collectCommands(dc devcontainer) []playbook.ReportedCommand {
	// Devcontainer lifecycle order: initialize → onCreate → updateContent →
	// postCreate → postStart → postAttach (waitFor is a hook NAME, not a command,
	// so it is intentionally not captured here as a command).
	hooks := []struct {
		name string
		raw  json.RawMessage
	}{
		{"initializeCommand", dc.InitializeCommand},
		{"onCreateCommand", dc.OnCreateCommand},
		{"updateContentCommand", dc.UpdateContentCommand},
		{"postCreateCommand", dc.PostCreateCommand},
		{"postStartCommand", dc.PostStartCommand},
		{"postAttachCommand", dc.PostAttachCommand},
	}
	var out []playbook.ReportedCommand
	for _, h := range hooks {
		if len(h.raw) == 0 {
			continue
		}
		out = append(out, playbook.ReportedCommand{Hook: h.name, Run: commandStrings(h.raw)})
	}
	return out
}

// commandStrings normalizes a devcontainer command value — which is string | array
// | object (parallel named commands) — into a flat list of literal command strings
// for human review. It NEVER executes anything; it only renders the value:
//   - "make build"                     -> ["make build"]
//   - ["npm", "install"]               -> ["npm install"]   (argv joined for reading)
//   - {"a": "cmd1", "b": ["x","y"]}    -> ["cmd1", "x y"]    (object values, key-sorted)
//
// A value it cannot interpret is preserved as its raw JSON token (lossless), never
// silently dropped. The strings are DISPLAY DATA only — quoting is not shell-safe and
// must never be fed to a shell (there is no execution path; this is the point).
func commandStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	// string form.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}
	}
	// array form (argv): join into one readable command line.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return []string{strings.Join(arr, " ")}
	}
	// object form: parallel named commands; render each value (key-sorted for
	// determinism), recursing so a named entry may itself be a string or argv array.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []string
		for _, k := range keys {
			out = append(out, commandStrings(obj[k])...)
		}
		return out
	}
	// Uninterpretable: preserve the raw token verbatim (lossless), trimmed.
	return []string{strings.TrimSpace(string(raw))}
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
