package engine

import (
	"fmt"
	"path/filepath"

	"github.com/iVersatile/loom/internal/lock"
	"github.com/iVersatile/loom/internal/playbook"
)

// Plan computes the diff between current state and the playbook; never mutates
// (docs/SPEC-verbs.md "plan"). It is the agent's gate before build: exit 2 when
// changes are needed, 0 when converged (the CLI maps PlanResult.Changed()).
func Plan(opts PlanOpts) (PlanResult, error) {
	return planImpl(opts, defaultRuntime())
}

// planImpl grades the CONTAINER, never the build host (LL-012: the guided run
// caught plan probing the host PATH while build converged the container —
// verdict and action disagreed, FR-PLAN-003). Three honest states:
//
//	absent container  -> create + every declared tool is an install (there is
//	                     no environment to grade yet)
//	running container -> live in-container probe (drift detection, ADR-0011)
//	stopped container -> the lock's container-pinned reality (T5); plan never
//	                     mutates, so it must not Start a container to ask
func planImpl(opts PlanOpts, rt ContainerRuntime) (PlanResult, error) {
	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}
	resolved, err := playbook.Load(path)
	if err != nil {
		return PlanResult{}, fmt.Errorf("plan requires a playbook (%s): %w", path, err)
	}
	pb := resolved.Playbook

	res := PlanResult{
		Create:  []CreateItem{},
		Install: []InstallItem{},
		Remove:  []RemoveItem{},
		Noop:    []string{},
	}

	// Container: if we cannot confirm it exists (absent, or no runtime), plan to
	// create it — build is idempotent, so a redundant create is a no-op. With no
	// container there is no environment to grade: every declared tool is work
	// build will do, regardless of what the HOST happens to have on PATH.
	cname := containerName(pb.Name)
	res.Target = cname
	if exists, err := rt.Exists(cname); err != nil || !exists {
		res.Create = append(res.Create, CreateItem{Kind: "container", Name: cname})
		for _, intent := range pb.Tools {
			name, want := playbook.SplitTool(intent)
			res.Install = append(res.Install, InstallItem{Tool: name, From: nil, To: playbook.WantOrLatest(want)})
		}
		return res, nil
	}

	running, _ := rt.Running(cname)
	var lockResolved map[string]string
	if !running {
		lockResolved = lockResolvedVersions(resolved.Root)
	}

	for _, intent := range pb.Tools {
		name, want := playbook.SplitTool(intent)
		var present bool
		var version string
		if running {
			present, version = rt.Probe(cname, playbook.BinaryName(name))
		} else {
			version = lockResolved[name]
			present = version != ""
		}
		switch {
		case !present:
			res.Install = append(res.Install, InstallItem{Tool: name, From: nil, To: playbook.WantOrLatest(want)})
		case !versionSatisfies(want, version):
			have := version
			res.Install = append(res.Install, InstallItem{Tool: name, From: &have, To: want})
		default:
			res.Noop = append(res.Noop, intent)
		}
	}

	// Real removal (a tool dropped from the playbook is uninstalled) is Phase 2
	// `update`; Remove stays empty in Phase 1 (docs/SPEC-verbs.md).
	return res, nil
}

// lockResolvedVersions reads the lock's container-probed `resolved` values
// (T5: the lock pins the container's reality, never the build host's).
// Unreadable/absent lock yields an empty map — conservative: plan reports
// the work, build settles it.
func lockResolvedVersions(root string) map[string]string {
	out := map[string]string{}
	lk, err := lock.Read(filepath.Join(root, "loom.lock"))
	if err != nil || lk == nil {
		return out
	}
	for name, t := range lk.Tools {
		out[name] = t.Resolved
	}
	return out
}
