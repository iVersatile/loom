package engine

import (
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

// TestImportReportsImageDefersFeaturesCommands pins FR-IMPORT-001's report/defer
// half: the image is REPORTED (never written into the playbook), and features +
// commands are DEFERRED, never silently dropped.
func TestImportReportsImageDefersFeaturesCommands(t *testing.T) {
	const image = "mcr.microsoft.com/devcontainers/go:1.22"
	src := writeFixture(t, `{
  "name": "demo",
  "image": "`+image+`",
  "features": {"ghcr.io/x/y:1": {}},
  "postCreateCommand": "make"
}`)

	res, err := Import(src)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if res.Reported["image"] != image {
		t.Errorf("Reported[image] = %q, want %q", res.Reported["image"], image)
	}
	if !slices.Contains(res.Deferred, "features") {
		t.Errorf("Deferred = %v, want it to contain features", res.Deferred)
	}
	if !slices.Contains(res.Deferred, "commands") {
		t.Errorf("Deferred = %v, want it to contain commands", res.Deferred)
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
