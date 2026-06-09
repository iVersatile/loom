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
func Doctor(opts DoctorOpts) (DoctorResult, error) {
	return doctorImpl(opts, execProber{})
}

func doctorImpl(opts DoctorOpts, p prober) (DoctorResult, error) {
	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}
	res := DoctorResult{Checks: []Check{}}

	resolved, err := playbook.Load(path)
	if err != nil {
		res.Checks = append(res.Checks, Check{Name: "playbook", OK: false, Detail: err.Error()})
		return res, nil
	}
	res.Checks = append(res.Checks, Check{Name: "playbook", OK: true, Detail: path})
	pb := resolved.Playbook

	// Required tools present on the host.
	for _, intent := range pb.Tools {
		name, _ := playbook.SplitTool(intent)
		present, version := p.probe(playbook.BinaryName(name))
		detail := "missing"
		if present {
			detail = version
		}
		res.Checks = append(res.Checks, Check{Name: "tool:" + name, OK: present, Detail: detail})
	}

	// Guardrail hooks present and executable (mechanism, not trust).
	for _, hs := range guard.Verify(resolved.Source, pb.Hooks) {
		detail := "ok"
		switch {
		case !hs.Present:
			detail = "missing from config source"
		case !hs.Executable:
			detail = "not executable"
		}
		res.Checks = append(res.Checks, Check{Name: "hook:" + hs.Name, OK: hs.OK(), Detail: detail})
	}

	// Lockfile present and covering the playbook's tools.
	l, err := lock.Read(filepath.Join(resolved.Root, "loom.lock"))
	switch {
	case err != nil:
		res.Checks = append(res.Checks, Check{Name: "lockfile", OK: false, Detail: err.Error()})
	case l == nil:
		res.Checks = append(res.Checks, Check{Name: "lockfile", OK: false, Detail: "loom.lock missing (run build)"})
	default:
		missing := toolsMissingFromLock(pb, l)
		detail := "consistent"
		if len(missing) > 0 {
			detail = fmt.Sprintf("not in lock: %v", missing)
		}
		res.Checks = append(res.Checks, Check{Name: "lockfile", OK: len(missing) == 0, Detail: detail})
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
