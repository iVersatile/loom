package lock

import (
	"path/filepath"
	"testing"
)

func sample() *Lock {
	return &Lock{
		LoomLock:   Version,
		ResolvedAt: "2026-06-08T00:00:00Z",
		BaseImage:  "debian:bookworm-slim@sha256:abc",
		Tools: map[string]LockedTool{
			"go": {Intent: "1.26", Resolved: "1.26.4", Source: "base-image"},
		},
		Agents: map[string]LockedAgent{
			"claude-code": {Resolved: "1.2.3"},
		},
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loom.lock")
	if err := sample().WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil || got.LoomLock != Version || got.BaseImage != "debian:bookworm-slim@sha256:abc" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Tools["go"].Resolved != "1.26.4" || got.Agents["claude-code"].Resolved != "1.2.3" {
		t.Errorf("nested fields lost: %+v", got)
	}
}

func TestReadMissingIsNotError(t *testing.T) {
	got, err := Read(filepath.Join(t.TempDir(), "absent.lock"))
	if err != nil || got != nil {
		t.Errorf("missing lock should be (nil,nil), got (%v,%v)", got, err)
	}
}

func TestContentEqualIgnoresTimestamp(t *testing.T) {
	a := sample()
	b := sample()
	b.ResolvedAt = "2099-01-01T00:00:00Z" // different timestamp only
	if !ContentEqual(a, b) {
		t.Error("locks differing only by ResolvedAt should be content-equal (idempotency)")
	}
	b.Tools["go"] = LockedTool{Intent: "1.27", Resolved: "1.27.0", Source: "base-image"}
	if ContentEqual(a, b) {
		t.Error("locks with different tool pins must not be content-equal")
	}
}
