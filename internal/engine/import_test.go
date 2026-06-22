package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/playbook"
)

// writeFixture writes a devcontainer.json into a fresh temp dir and returns its
// path. Tests never write into the repo tree.
func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "devcontainer.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestImportMapsPortsAndEnv pins FR-IMPORT-001's mapping half: forwardPorts +
// appPort -> ports (int and string forms), containerEnv ∪ remoteEnv -> env names
// (deduped, sorted), and the draft re-parses + validates as a project playbook.
func TestImportMapsPortsAndEnv(t *testing.T) {
	src := writeFixture(t, `{
  "name": "demo",
  "image": "mcr.microsoft.com/devcontainers/go:1.22",
  "forwardPorts": [3000, "8080"],
  "containerEnv": {"NODE_ENV": "production"},
  "remoteEnv": {"API_HOST": "x"}
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if !reflect.DeepEqual(res.Mapped.Ports, []int{3000, 8080}) {
		t.Errorf("Mapped.Ports = %v, want [3000 8080]", res.Mapped.Ports)
	}
	if !reflect.DeepEqual(res.Mapped.Env, []string{"API_HOST", "NODE_ENV"}) {
		t.Errorf("Mapped.Env = %v, want [API_HOST NODE_ENV]", res.Mapped.Env)
	}

	if _, err := os.Stat(res.Draft); err != nil {
		t.Fatalf("draft not written: %v", err)
	}
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if err := pb.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if pb.Name == "" {
		t.Errorf("draft Name is empty, want a project name")
	}
	if pb.Name != "demo" {
		t.Errorf("Name = %q, want demo (devcontainer.name)", pb.Name)
	}
	if !reflect.DeepEqual(pb.Ports, []int{3000, 8080}) {
		t.Errorf("draft Ports = %v, want [3000 8080]", pb.Ports)
	}
	if !reflect.DeepEqual(pb.Env, []string{"API_HOST", "NODE_ENV"}) {
		t.Errorf("draft Env = %v, want [API_HOST NODE_ENV]", pb.Env)
	}
}

// TestImportReportsImageAndCommands pins FR-IMPORT-001's report half (now that BOTH
// features and commands are REPORTED, never deferred): the image is REPORTED (never
// written into the playbook); a CUSTOM/unmapped feature is REPORTED (unmapped_features);
// the lifecycle command is REPORTED (its hook surfaced in --json reported.commands);
// and NOTHING remains in the deferred set — every non-mapped field is reported, never
// silently dropped.
func TestImportReportsImageAndCommands(t *testing.T) {
	const image = "mcr.microsoft.com/devcontainers/go:1.22"
	const customRef = "ghcr.io/acme/custom:1"
	src := writeFixture(t, `{
  "name": "demo",
  "image": "`+image+`",
  "features": {"`+customRef+`": {}},
  "postCreateCommand": "make"
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if res.Reported["image"] != image {
		t.Errorf("Reported[image] = %q, want %q", res.Reported["image"], image)
	}
	if slices.Contains(res.Deferred, "features") {
		t.Errorf("Deferred = %v, want it to NOT contain features (features now map/report)", res.Deferred)
	}
	// commands are now REPORTED, not deferred: the deferred set is empty.
	if slices.Contains(res.Deferred, "commands") {
		t.Errorf("Deferred = %v, want it to NOT contain commands (commands are now REPORTED)", res.Deferred)
	}
	if len(res.Deferred) != 0 {
		t.Errorf("Deferred = %v, want empty (nothing remains deferred in Stage-1 import)", res.Deferred)
	}
	if !strings.Contains(res.Reported["commands"], "postCreateCommand") {
		t.Errorf("Reported[commands] = %q, want it to surface the postCreateCommand hook", res.Reported["commands"])
	}
	if !strings.Contains(res.Reported["unmapped_features"], customRef) {
		t.Errorf("Reported[unmapped_features] = %q, want it to contain %q", res.Reported["unmapped_features"], customRef)
	}

	// The image must NEVER be written into the playbook (base_image is an
	// engine-level floor; loom enriches, never degrades-to — ADR-0012/0003).
	data, err := os.ReadFile(res.Draft)
	if err != nil {
		t.Fatalf("ReadFile draft: %v", err)
	}
	if strings.Contains(string(data), image) {
		t.Errorf("draft leaked the image %q:\n%s", image, data)
	}
}

// TestImportMapsKnownFeaturesToTools pins FR-IMPORT-004's mapping half: recognized
// official features map to loom tools — name@<version-option> when the feature's
// `version` OPTION is a clean token, else the bare tool name — deduped + sorted, and
// `features` leaves the deferred set.
func TestImportMapsKnownFeaturesToTools(t *testing.T) {
	src := writeFixture(t, `{
  "name": "demo",
  "features": {
    "ghcr.io/devcontainers/features/go:1": {"version": "1.22"},
    "ghcr.io/devcontainers/features/node:1": {}
  }
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	want := []string{"go@1.22", "node"}
	if !reflect.DeepEqual(res.Mapped.Tools, want) {
		t.Errorf("Mapped.Tools = %v, want %v", res.Mapped.Tools, want)
	}
	if slices.Contains(res.Deferred, "features") {
		t.Errorf("Deferred = %v, want it to NOT contain features", res.Deferred)
	}

	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if !reflect.DeepEqual(pb.Tools, want) {
		t.Errorf("draft Tools = %v, want %v", pb.Tools, want)
	}
}

// TestImportReportsUnknownFeatures pins FR-IMPORT-004's report half: a recognized
// feature still maps to a tool while an UNRECOGNIZED/custom ref is surfaced in
// reported (unmapped_features), never guessed; `features` stays out of deferred.
func TestImportReportsUnknownFeatures(t *testing.T) {
	const customRef = "ghcr.io/acme/custom-thing:2"
	src := writeFixture(t, `{
  "name": "demo",
  "features": {
    "ghcr.io/devcontainers/features/go:1": {},
    "`+customRef+`": {}
  }
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if !reflect.DeepEqual(res.Mapped.Tools, []string{"go"}) {
		t.Errorf("Mapped.Tools = %v, want [go]", res.Mapped.Tools)
	}
	if !strings.Contains(res.Reported["unmapped_features"], customRef) {
		t.Errorf("Reported[unmapped_features] = %q, want it to contain %q", res.Reported["unmapped_features"], customRef)
	}
	if slices.Contains(res.Deferred, "features") {
		t.Errorf("Deferred = %v, want it to NOT contain features", res.Deferred)
	}
}

// TestImportFeatureNameBoundary pins the host-anchored boundary match (not substring):
// a CUSTOM ref that merely CONTAINS "devcontainers/features/" but isn't the official
// <host>/devcontainers/features/<name> shape must be REPORTED, never wrongly mapped to
// the official tool (the false-positive direction is the unsafe one).
func TestImportFeatureNameBoundary(t *testing.T) {
	const fake1 = "ghcr.io/me/devcontainers/features/go:1" // 5 segments — extra path
	const fake2 = "x-devcontainers/features/go:1"          // 3 segments — substring, not a host boundary
	src := writeFixture(t, `{
  "name": "demo",
  "features": { "`+fake1+`": {}, "`+fake2+`": {} }
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Mapped.Tools) != 0 {
		t.Errorf("Mapped.Tools = %v, want empty (custom refs must NOT masquerade as official go)", res.Mapped.Tools)
	}
	um := res.Reported["unmapped_features"]
	if !strings.Contains(um, fake1) || !strings.Contains(um, fake2) {
		t.Errorf("both custom refs must be reported as unmapped, got %q", um)
	}
}

// TestImportMultiToolFeatureBareNames pins F3: a multi-tool feature
// (kubectl-helm-minikube -> kubectl, helm) emits BARE names — the single `version`
// option pins only the primary tool, so applying it to siblings would be a wrong pin.
func TestImportMultiToolFeatureBareNames(t *testing.T) {
	src := writeFixture(t, `{
  "name": "demo",
  "features": { "ghcr.io/devcontainers/features/kubectl-helm-minikube:1": {"version": "1.30"} }
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	want := []string{"helm", "kubectl"}
	if !reflect.DeepEqual(res.Mapped.Tools, want) {
		t.Errorf("Mapped.Tools = %v, want %v (bare names; the version is tool-ambiguous)", res.Mapped.Tools, want)
	}
}

// TestImportDraftValidatesNoClobber pins FR-IMPORT-002's no-clobber output: the
// draft basename is loom.imported.yml (never loom.yml), and a pre-existing
// authored loom.yml in the same dir is left untouched.
func TestImportDraftValidatesNoClobber(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "devcontainer.json")
	if err := os.WriteFile(src, []byte(`{"name":"demo","forwardPorts":[3000]}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	const sentinel = "# AUTHORED - DO NOT TOUCH\nloom: 1\n"
	canonical := filepath.Join(dir, "loom.yml")
	if err := os.WriteFile(canonical, []byte(sentinel), 0o644); err != nil {
		t.Fatalf("write sentinel loom.yml: %v", err)
	}

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if filepath.Base(res.Draft) != importedPlaybookName {
		t.Errorf("draft basename = %q, want %q", filepath.Base(res.Draft), importedPlaybookName)
	}
	if filepath.Base(res.Draft) == "loom.yml" {
		t.Errorf("draft must never be loom.yml, got %q", res.Draft)
	}

	got, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("ReadFile loom.yml: %v", err)
	}
	if string(got) != sentinel {
		t.Errorf("authored loom.yml was modified:\n%s", got)
	}
}

// TestImportStripsJSONCCommentsRespectingStrings pins FR-IMPORT-003: import
// tolerates // line comments, /* block */ comments, and trailing commas, WITHOUT
// mis-stripping comment-like sequences inside string values.
func TestImportStripsJSONCCommentsRespectingStrings(t *testing.T) {
	src := writeFixture(t, `{
  // a line comment
  "name": "app // not-a-comment",
  /* a block comment */
  "image": "https://example.com/img", // trailing comment
  "containerEnv": {
    "HOMEPAGE": "https://example.com",
    "CSV": "a,b",
  },
  "forwardPorts": [3000, 8080,],
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import (JSONC should be tolerated): %v", err)
	}
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	// The //-inside-a-string must have survived the stripper (the footgun): the
	// image is REPORTED verbatim, so its `//` proves it directly.
	if res.Reported["image"] != "https://example.com/img" {
		t.Errorf("reported image = %q, want %q (//-inside-string survived verbatim)", res.Reported["image"], "https://example.com/img")
	}
	// Secondary probe: the `//` inside the name value also survived — "app //
	// not-a-comment" slugifies to "app-not-a-comment"; had the stripper eaten the
	// "//" as a comment, the value would be "app " → slug "app".
	if pb.Name != "app-not-a-comment" {
		t.Errorf("Name = %q, want %q (//-inside-name survived the stripper, then slugified)", pb.Name, "app-not-a-comment")
	}
	if !reflect.DeepEqual(pb.Env, []string{"CSV", "HOMEPAGE"}) {
		t.Errorf("Env = %v, want [CSV HOMEPAGE]", pb.Env)
	}
	if !reflect.DeepEqual(pb.Ports, []int{3000, 8080}) {
		t.Errorf("Ports = %v, want [3000 8080] (trailing comma tolerated)", pb.Ports)
	}
}

// TestStripJSONC directly asserts the helper preserves comment-like/comma-like
// sequences inside strings, removes comments outside strings, and drops trailing
// commas.
func TestStripJSONC(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"line comment outside", "{\n// gone\n\"a\":1\n}", "{\n\n\"a\":1\n}"},
		{"block comment outside", "{/* gone */\"a\":1}", "{\"a\":1}"},
		{"slashes inside string", `{"u":"http://x//y"}`, `{"u":"http://x//y"}`},
		{"block marker inside string", `{"u":"a/*b*/c"}`, `{"u":"a/*b*/c"}`},
		{"comma inside string", `{"u":"a,b"}`, `{"u":"a,b"}`},
		{"trailing comma object", `{"a":1,}`, `{"a":1}`},
		{"trailing comma array", `[1,2,]`, `[1,2]`},
		{"trailing comma before comment", "[1,2, /*x*/ ]", "[1,2  ]"},
		{"escaped quote in string", `{"u":"a\"//b"}`, `{"u":"a\"//b"}`},
		{"non-trailing comma kept", `{"a":1,"b":2}`, `{"a":1,"b":2}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(stripJSONC([]byte(c.in))); got != c.want {
				t.Errorf("stripJSONC(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestImportSanitizesName proves a devcontainer name with docker-unsafe chars
// (spaces, '/', ':') is slugified into a valid playbook/container name — an
// unsanitized name would flow into the playbook `name:` and break the build.
func TestImportSanitizesName(t *testing.T) {
	src := writeFixture(t, `{"name":"My Org/Repo: v2","image":"x"}`)
	res, err := Import(src)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if pb.Name != "my-org-repo-v2" {
		t.Errorf("Name = %q, want %q (slugified)", pb.Name, "my-org-repo-v2")
	}
	if err := pb.Validate(); err != nil {
		t.Errorf("sanitized draft must validate: %v", err)
	}
}

// executableFields are the playbook fields the engine actually PROVISIONS or runs
// from. FR-IMPORT-005's security guarantee is that a captured command lands in NONE
// of them — it is review DATA in reported.commands, never an execution path. Asserted
// by re-marshaling only these fields and confirming the command string is absent.
type executableFields struct {
	Tools    []string `json:"tools"`
	Agents   []string `json:"agents"`
	Hooks    []string `json:"hooks"`
	Rules    []string `json:"rules"`
	Dotfiles []string `json:"dotfiles"`
}

// TestImportCapturesLifecycleCommandsAsReported pins FR-IMPORT-005's core: a
// devcontainer with a postCreateCommand (string form) captures it verbatim into the
// draft's reported.commands tagged with its lifecycle hook — and the command appears
// in NO executable/mapped field (tools/agents/hooks/rules/dotfiles), proving loom
// never wires it for execution (ADR-0005 worst-thing test). loom never auto-runs it.
func TestImportCapturesLifecycleCommandsAsReported(t *testing.T) {
	const cmd = "make build && ./dangerous.sh"
	src := writeFixture(t, `{
  "name": "demo",
  "postCreateCommand": "`+cmd+`"
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if pb.Reported == nil || len(pb.Reported.Commands) != 1 {
		t.Fatalf("draft reported.commands = %+v, want exactly one captured command", pb.Reported)
	}
	got := pb.Reported.Commands[0]
	if got.Hook != "postCreateCommand" {
		t.Errorf("captured hook = %q, want postCreateCommand", got.Hook)
	}
	if !reflect.DeepEqual(got.Run, []string{cmd}) {
		t.Errorf("captured Run = %v, want [%q] (verbatim)", got.Run, cmd)
	}

	// The command must NOT appear in any field the engine provisions/runs from.
	exec, err := json.Marshal(executableFields{
		Tools: pb.Tools, Agents: pb.Agents, Hooks: pb.Hooks, Rules: pb.Rules, Dotfiles: pb.Dotfiles,
	})
	if err != nil {
		t.Fatalf("marshal executable fields: %v", err)
	}
	if strings.Contains(string(exec), "make") || strings.Contains(string(exec), "dangerous") {
		t.Errorf("imported command leaked into an executable field: %s", exec)
	}
	// And it must NOT be deferred — it has a (non-executable) home.
	if slices.Contains(res.Deferred, "commands") {
		t.Errorf("Deferred = %v, want commands REPORTED not deferred", res.Deferred)
	}
}

// TestImportCapturesCommandFormsArrayObject pins FR-IMPORT-005's polymorphic handling:
// the devcontainer array form (argv) and object form (parallel named commands) are
// both captured into reported.commands (normalized to readable command strings), in
// lifecycle order, none mapped to an executable field.
func TestImportCapturesCommandFormsArrayObject(t *testing.T) {
	src := writeFixture(t, `{
  "name": "demo",
  "onCreateCommand": ["npm", "ci"],
  "postStartCommand": { "server": "node server.js", "watch": ["npm", "run", "watch"] }
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if pb.Reported == nil {
		t.Fatalf("draft reported is nil, want captured commands")
	}
	// Lifecycle order: onCreateCommand before postStartCommand.
	want := []playbook.ReportedCommand{
		{Hook: "onCreateCommand", Run: []string{"npm ci"}},
		{Hook: "postStartCommand", Run: []string{"node server.js", "npm run watch"}}, // object keys sorted: server, watch
	}
	if !reflect.DeepEqual(pb.Reported.Commands, want) {
		t.Errorf("reported.commands = %+v, want %+v", pb.Reported.Commands, want)
	}
	if len(res.Deferred) != 0 {
		t.Errorf("Deferred = %v, want empty", res.Deferred)
	}
}

// TestImportReportedCommandsRoundTripStrictDecode pins FR-IMPORT-005's strict-decode
// safety (#256): an imported draft carrying reported.commands re-parses CLEANLY under
// the strict (DisallowUnknownFields) decoder — i.e. reported.commands is a declared
// schema field, not a key strict decode would reject — and validates as a project-tier
// playbook. This is the round-trip guarantee: import-then-parse never breaks.
func TestImportReportedCommandsRoundTripStrictDecode(t *testing.T) {
	src := writeFixture(t, `{
  "name": "demo",
  "initializeCommand": "echo init",
  "postCreateCommand": "make"
}`)
	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// ParseFile uses the STRICT decoder (UnmarshalStrict). If reported.commands were
	// not a declared field, this would fail with "unknown field" — the whole point.
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("strict re-parse of imported draft failed (reported.commands must be a declared field): %v", err)
	}
	if err := pb.Validate(); err != nil {
		t.Fatalf("re-parsed draft must validate: %v", err)
	}
	if pb.Reported == nil || len(pb.Reported.Commands) != 2 {
		t.Fatalf("round-tripped reported.commands = %+v, want 2 captured commands", pb.Reported)
	}
}
