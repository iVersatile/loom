package source

import (
	"io/fs"
	"strings"
	"testing"
)

func TestResolveLocalReadsTree(t *testing.T) {
	// The playbook package ships a config tree under its testdata; reuse it via a
	// relative path from this package to exercise local resolution + reads.
	root := "../playbook/testdata/proj"
	fsys, err := Resolve(root, "local", "./config")
	if err != nil {
		t.Fatalf("resolve local: %v", err)
	}
	if _, err := fs.ReadFile(fsys, "playbook.yml"); err != nil {
		t.Errorf("expected to read base playbook.yml from source: %v", err)
	}
}

func TestResolveGitUnsupported(t *testing.T) {
	_, err := Resolve(".", "git", "")
	if err == nil || !strings.Contains(err.Error(), "not yet supported") {
		t.Errorf("git source should be an explicit unsupported error, got %v", err)
	}
}

func TestResolvePathEscapeRejected(t *testing.T) {
	_, err := Resolve("../playbook/testdata/proj", "local", "../../../../etc")
	if err == nil || !strings.Contains(err.Error(), "escapes project root") {
		t.Errorf("path escaping root should be rejected, got %v", err)
	}
}

func TestResolveMissingDir(t *testing.T) {
	if _, err := Resolve(".", "local", "./does-not-exist"); err == nil {
		t.Error("missing config dir should error")
	}
}

func TestResolveUnknownType(t *testing.T) {
	if _, err := Resolve(".", "svn", "x"); err == nil {
		t.Error("unknown source type should error")
	}
}
