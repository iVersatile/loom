package engine

import (
	"path/filepath"
	"testing"

	"github.com/iVersatile/loom/internal/lock"
)

// TestUpdateAppliesDeltaConverges (FR-UPDATE-001): update reconciles a running env
// to the current playbook — it computes the plan delta, applies the additions
// (create/install) via the shared provision path, and rewrites loom.lock. On a
// fresh project (no container, no lock) the whole declared set is the delta, so
// the result is "converged".
func TestUpdateAppliesDeltaConverges(t *testing.T) {
	root := tempProject(t)
	rt := fakeRuntime{ensureInfo: ContainerInfo{Name: "loom-dev", Status: "created"}, probeVersions: fixtureProbes()}
	res, err := updateImpl(UpdateOpts{PlaybookPath: filepath.Join(root, "loom.yml")}, rt, fixedClock)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res.Result != "converged" {
		t.Errorf("result = %q, want converged", res.Result)
	}
	if !res.LockWritten {
		t.Error("update should rewrite loom.lock when applying a delta")
	}
	if len(res.Install) == 0 || len(res.Create) == 0 {
		t.Errorf("delta empty: install=%+v create=%+v, want the fresh-project additions", res.Install, res.Create)
	}
	if len(res.Actions) == 0 {
		t.Error("a converged update must append audit entries")
	}
	// The reconcile actually wrote loom.lock to disk (re-pinned from the container).
	if _, err := lock.Read(filepath.Join(root, "loom.lock")); err != nil {
		t.Errorf("loom.lock not written: %v", err)
	}
}

// TestUpdateNoopWhenConverged (FR-UPDATE-002): when the env already matches the
// playbook (no delta), update is a noop — no lock rewrite, no audit entries, an
// empty delta. This makes the result enum's "noop" reachable (review-finding R6);
// build never returns it. Built host side + a fully-converged container = no work.
func TestUpdateNoopWhenConverged(t *testing.T) {
	pbPath, root := builtProject(t, fixtureProbes())
	before := countLogLines(t, root)
	converged := fakeRuntime{
		exists: true, running: true,
		probeVersions: fixtureProbes(),
		homeSentinel:  homeDigest(filepath.Join(root, ".loom", "home")),
	}
	res, err := updateImpl(UpdateOpts{PlaybookPath: pbPath}, converged, fixedClock)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res.Result != "noop" {
		t.Errorf("result = %q, want noop (env already matches the playbook)", res.Result)
	}
	if res.LockWritten {
		t.Error("a noop update must not rewrite loom.lock")
	}
	if len(res.Install) != 0 || len(res.Create) != 0 {
		t.Errorf("noop must carry no delta: install=%+v create=%+v", res.Install, res.Create)
	}
	if after := countLogLines(t, root); after != before {
		t.Errorf("noop update appended %d audit entries, want 0", after-before)
	}
}

// TestUpdateForceBypassesNoop: `update --force` reconciles REGARDLESS of whether
// there is a delta (mirrors `build --force`). The noop gate is bypassed when Force
// is set, so a fully-converged env still does the forced reconcile — Result is
// "converged", not "noop", and the work is audited.
func TestUpdateForceBypassesNoop(t *testing.T) {
	pbPath, root := builtProject(t, fixtureProbes())
	before := countLogLines(t, root)
	// Same fully-converged setup as TestUpdateNoopWhenConverged (a plain update
	// would noop), but Force:true — and Ensure recreates the container (force).
	rt := fakeRuntime{
		exists: true, running: true,
		probeVersions: fixtureProbes(),
		homeSentinel:  homeDigest(filepath.Join(root, ".loom", "home")),
		ensureInfo:    ContainerInfo{Name: "loom-dev", Status: "created"},
	}
	res, err := updateImpl(UpdateOpts{PlaybookPath: pbPath, Force: true}, rt, fixedClock)
	if err != nil {
		t.Fatalf("update --force: %v", err)
	}
	if res.Result == "noop" {
		t.Error("update --force on a converged env must not noop")
	}
	if res.Result != "converged" {
		t.Errorf("result = %q, want converged (force did the reconcile)", res.Result)
	}
	if after := countLogLines(t, root); after <= before {
		t.Errorf("forced reconcile appended %d audit entries, want > 0", after-before)
	}
}
