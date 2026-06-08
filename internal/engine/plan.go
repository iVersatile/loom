package engine

import (
	"fmt"

	"github.com/iVersatile/loom/internal/playbook"
)

// Plan computes the diff between current state and the playbook; never mutates
// (docs/SPEC-verbs.md "plan"). It is the agent's gate before build: exit 2 when
// changes are needed, 0 when converged (the CLI maps PlanResult.Changed()).
func Plan(opts PlanOpts) (PlanResult, error) {
	return planImpl(opts, execProber{}, defaultRuntime())
}

func planImpl(opts PlanOpts, p prober, rt ContainerRuntime) (PlanResult, error) {
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
	// create it — build is idempotent, so a redundant create is a no-op.
	cname := containerName(pb.Name)
	if exists, err := rt.Exists(cname); err != nil || !exists {
		res.Create = append(res.Create, CreateItem{Kind: "container", Name: cname})
	}

	for _, intent := range pb.Tools {
		name, want := playbook.SplitTool(intent)
		present, version := p.probe(binaryName(name))
		switch {
		case !present:
			res.Install = append(res.Install, InstallItem{Tool: name, From: nil, To: wantOrLatest(want)})
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
