package engine

import (
	"fmt"
	"time"
)

// Update reconciles a running environment to the CURRENT (edited) playbook by
// applying the plan delta (docs/SPEC-verbs.md "update"). Slice 1 applies the
// additions (create/install) and re-pins loom.lock; real removal is deferred to
// Slice 2 (the planner leaves Plan.Remove empty today, plan.go). The result is
// "noop" when the env already matches the playbook, "converged" when a delta was
// applied.
func Update(opts UpdateOpts) (UpdateResult, error) {
	return updateImpl(opts, defaultRuntime(), time.Now)
}

func updateImpl(opts UpdateOpts, rt ContainerRuntime, now func() time.Time) (UpdateResult, error) {
	// 1. Compute the delta with the SAME planner `plan` uses — verdict and action
	//    share every graded dimension (the engine's one grader; no second opinion).
	plan, err := planImpl(PlanOpts{PlaybookPath: opts.PlaybookPath}, rt)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: plan: %w", err)
	}

	// 2. No delta → noop. Side-effect-free and idempotent: a converged env writes
	//    nothing (no lock rewrite, no audit entry), so a 2nd `update` is a clean
	//    no-op (FR-INV-003). This is the reachable "noop" the result enum needs (R6).
	if !plan.Changed() {
		return UpdateResult{Result: "noop", Target: plan.Target}, nil
	}

	// 3. Apply the delta via the shared build/provision path — it creates/installs
	//    the additions, re-resolves, and rewrites loom.lock, stamping THIS verb's
	//    identity in the audit + diagnostic logs (buildImplVerb). Slice 1: Plan.Remove
	//    is empty, so the converge path IS the full apply; Slice 2 adds real uninstall.
	br, err := buildImplVerb(BuildOpts(opts), rt, now, "update")
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: apply delta: %w", err)
	}

	// The pre-apply plan IS the delta update reconciled (additions only in Slice 1).
	return UpdateResult{
		Result:      "converged",
		Target:      br.Container.Name,
		Install:     plan.Install,
		Create:      plan.Create,
		LockWritten: br.LockWritten,
		Actions:     br.Actions,
		LogPath:     br.LogPath,
		Warnings:    br.Warnings,
	}, nil
}
