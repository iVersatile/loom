package engine

import (
	"fmt"
	"path/filepath"

	"github.com/iVersatile/loom/internal/guard"
	"github.com/iVersatile/loom/internal/lock"
	"github.com/iVersatile/loom/internal/playbook"
)

// Doctor self-checks the environment (docs/SPEC-verbs.md "doctor / verify"):
// required tools present, guardrail hooks executable, lockfile consistent with
// the playbook. It reports via checks rather than hard-failing; the CLI maps a
// failing check set to a non-zero exit.
//
// Every check NAMES the tier it grades (guided-run finding ⑧, LL-012 class:
// doctor once answered tool checks from the host PATH while exec hit the
// container — a bill of health must say which environment it is a bill of
// health FOR):
//
//	host:*      — repo-side facts: playbook parses, hooks executable, lockfile
//	              consistent. True regardless of any container.
//	container:* — the container's reality. Tools are probed live when the
//	              container runs; a stopped container is graded from the
//	              lock's container-pinned resolved versions (T5 — doctor never
//	              mutates, so it never Starts a container to ask). With no
//	              container there is nothing to grade: container:state fails
//	              with the remedy and per-tool checks are omitted rather than
//	              invented from the host PATH.
func Doctor(opts DoctorOpts) (DoctorResult, error) {
	return doctorImpl(opts, defaultRuntime())
}

func doctorImpl(opts DoctorOpts, rt ContainerRuntime) (DoctorResult, error) {
	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}
	res := DoctorResult{Checks: []Check{}}

	resolved, err := playbook.Load(path)
	if err != nil {
		res.Checks = append(res.Checks, Check{Name: "host:playbook", OK: false, Detail: err.Error()})
		return res, nil
	}
	res.Checks = append(res.Checks, Check{Name: "host:playbook", OK: true, Detail: path})
	pb := resolved.Playbook

	// Guardrail hooks present and executable (mechanism, not trust).
	for _, hs := range guard.Verify(resolved.Source, pb.Hooks) {
		detail := "ok"
		switch {
		case !hs.Present:
			detail = "missing from config source"
		case !hs.Executable:
			detail = "not executable"
		}
		res.Checks = append(res.Checks, Check{Name: "host:hook:" + hs.Name, OK: hs.OK(), Detail: detail})
	}

	// Lockfile present and covering the playbook's tools.
	l, err := lock.Read(filepath.Join(resolved.Root, "loom.lock"))
	switch {
	case err != nil:
		res.Checks = append(res.Checks, Check{Name: "host:lockfile", OK: false, Detail: err.Error()})
	case l == nil:
		res.Checks = append(res.Checks, Check{Name: "host:lockfile", OK: false, Detail: "loom.lock missing (run build)"})
	default:
		missing := toolsMissingFromLock(pb, l)
		detail := "consistent"
		if len(missing) > 0 {
			detail = fmt.Sprintf("not in lock: %v", missing)
		}
		res.Checks = append(res.Checks, Check{Name: "host:lockfile", OK: len(missing) == 0, Detail: detail})
	}

	// Container tier: state first; tools are graded only against a real
	// container, never the host PATH.
	cname := containerName(pb.Name)
	exists, eerr := rt.Exists(cname)
	switch {
	case eerr != nil:
		res.Checks = append(res.Checks, Check{Name: "container:state", OK: false,
			Detail: fmt.Sprintf("%s: runtime unavailable (%v)", cname, eerr)})
		return res, nil
	case !exists:
		res.Checks = append(res.Checks, Check{Name: "container:state", OK: false,
			Detail: cname + ": absent (run build)"})
		return res, nil
	}

	running, _ := rt.Running(cname)
	state := "running (live probe)"
	var lockResolved map[string]string
	if !running {
		state = "stopped (tool versions from lock)"
		lockResolved = lockResolvedVersions(resolved.Root)
	}
	res.Checks = append(res.Checks, Check{Name: "container:state", OK: true, Detail: cname + ": " + state})

	for _, intent := range pb.Tools {
		name, _ := playbook.SplitTool(intent)
		var present bool
		var version string
		if running {
			present, version = rt.Probe(cname, playbook.BinaryName(name))
		} else {
			version = lockResolved[name]
			present = version != ""
		}
		detail := "missing"
		if present {
			detail = version
		}
		res.Checks = append(res.Checks, Check{Name: "container:tool:" + name, OK: present, Detail: detail})
	}

	return res, nil
}

func toolsMissingFromLock(pb *playbook.Playbook, l *lock.Lock) []string {
	var missing []string
	for _, intent := range pb.Tools {
		name, _ := playbook.SplitTool(intent)
		if _, ok := l.Tools[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}
