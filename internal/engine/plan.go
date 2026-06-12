package engine

import (
	"fmt"
	"path/filepath"

	"github.com/iVersatile/loom/internal/lock"
	"github.com/iVersatile/loom/internal/playbook"
	"github.com/iVersatile/loom/internal/resolver"
)

// Plan computes the diff between current state and the playbook; never mutates
// (docs/SPEC-verbs.md "plan"). It is the agent's gate before build: exit 2 when
// changes are needed, 0 when converged (the CLI maps PlanResult.Changed()).
func Plan(opts PlanOpts) (PlanResult, error) {
	return planImpl(opts, defaultRuntime())
}

// planImpl grades every dimension build converges, never the build host
// (LL-012 / phase-1 review F2: plan once graded container+tools only while
// build also converged the lockfile, the staged $HOME, and the agent set —
// "converged" and "would mutate" disagreed on three dimensions).
//
// Host-side dimensions (true regardless of any container):
//
//	lockfile    -> would build rewrite loom.lock? (same resolve + carry-forward
//	               path build runs, compared content-equal)
//	staged home -> would build re-materialize a dotfile/harness artifact?
//	               (read-only dry run against the config source)
//
// Container-side dimensions (the LL-012 rules unchanged):
//
//	absent container  -> create + every declared tool AND agent is an install
//	                     (there is no environment to grade yet)
//	running container -> live in-container probe (drift detection, ADR-0011),
//	                     plus the home-sync sentinel vs the staged digest
//	stopped container -> the lock's container-pinned reality (T5); plan never
//	                     mutates, so it must not Start a container to ask; the
//	                     home sentinel is unreadable stopped, so home wiring is
//	                     graded at the staging tier only
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
	root := resolved.Root

	res := PlanResult{
		Create:  []CreateItem{},
		Install: []InstallItem{},
		Remove:  []RemoveItem{},
		Noop:    []string{},
	}
	cname := containerName(pb.Name)
	res.Target = cname

	// Lockfile dimension (F2): the same resolve + carry-forward + base-digest
	// path build runs, compared content-equal — an unreadable or absent lock
	// reads as nil, exactly what build would rewrite.
	existing, lerr := lock.Read(filepath.Join(root, "loom.lock"))
	if lerr != nil {
		existing = nil
	}
	resolution := resolver.Resolve(pb, nullVersions{})
	carryForwardResolved(resolution, existing)
	img := baseImage()
	if digest, derr := rt.ResolveBaseDigest(img); derr == nil && digest != "" {
		img = img + "@" + digest
	}
	if lock.ContentEqual(existing, resolution.Lock(img, "")) {
		res.Noop = append(res.Noop, "lockfile")
	} else {
		res.Create = append(res.Create, CreateItem{Kind: "lockfile", Name: "loom.lock"})
	}

	// Staged-home dimension (F2): a read-only dry run of materialize, over
	// the same expected set build writes — declared dotfiles + harness + the
	// engine-generated PATH dotfiles (T4).
	staging := filepath.Join(root, ".loom", "home")
	generated := expectedPathDotfiles(toolInstalls(resolution), agentInstalls(resolution), staging)
	drift, derr := stagedHomeDrift(resolved.Source, pb, generated, staging)
	if derr != nil {
		return res, fmt.Errorf("plan: grade staged home: %w", derr)
	}
	for _, d := range drift {
		res.Create = append(res.Create, CreateItem{Kind: "dotfile", Name: d})
	}
	stagedClean := len(drift) == 0
	if stagedClean && len(pb.Dotfiles)+len(pb.Harness)+len(generated) > 0 {
		res.Noop = append(res.Noop, "home")
	}

	// Container: if we cannot confirm it exists (absent, or no runtime), plan to
	// create it — build is idempotent, so a redundant create is a no-op. With no
	// container there is no environment to grade: every declared tool AND agent
	// is work build will do, regardless of what the HOST happens to have on PATH.
	if exists, err := rt.Exists(cname); err != nil || !exists {
		res.Create = append(res.Create, CreateItem{Kind: "container", Name: cname})
		for _, intent := range pb.Tools {
			name, want := playbook.SplitTool(intent)
			res.Install = append(res.Install, InstallItem{Tool: name, From: nil, To: playbook.WantOrLatest(want)})
		}
		for _, a := range pb.Agents {
			res.Install = append(res.Install, InstallItem{Tool: "agent:" + a, From: nil, To: "latest"})
		}
		return res, nil
	}

	running, _ := rt.Running(cname)
	lockResolved := map[string]string{}
	if !running && existing != nil {
		for name, t := range existing.Tools {
			lockResolved[name] = t.Resolved
		}
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

	// Agent dimension (F2): build provisions the declared agent set; plan
	// grades it the same way as tools — live probe running, lock pin stopped.
	for _, a := range pb.Agents {
		var present bool
		if running {
			present, _ = rt.Probe(cname, playbook.BinaryName(a))
		} else if existing != nil {
			la, ok := existing.Agents[a]
			present = ok && la.Resolved != ""
		}
		if present {
			res.Noop = append(res.Noop, "agent:"+a)
		} else {
			res.Install = append(res.Install, InstallItem{Tool: "agent:" + a, From: nil, To: "latest"})
		}
	}

	// Home-sync dimension (F2): a clean staging dir can still be ahead of the
	// container (interrupted sync, pre-T7 container) — build would re-sync, so
	// plan must say so. Graded only against a running container's sentinel;
	// when staging itself drifts the dotfile items above already flag the work.
	if running && stagedClean {
		if want := homeDigest(staging); want != "" {
			if rt.HomeDigest(cname) == want {
				res.Noop = append(res.Noop, "home-sync")
			} else {
				res.Create = append(res.Create, CreateItem{Kind: "home-sync", Name: cname})
			}
		}
	}

	// Real removal (a tool dropped from the playbook is uninstalled) is Phase 2
	// `update`; Remove stays empty in Phase 1 (docs/SPEC-verbs.md).
	return res, nil
}

// lockResolvedVersions reads the lock's container-probed `resolved` values
// (T5: the lock pins the container's reality, never the build host's).
// Unreadable/absent lock yields an empty map — conservative: the caller
// reports the work, build settles it.
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
