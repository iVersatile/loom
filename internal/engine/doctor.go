package engine

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

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

	// Guardrails WIRED, not just present (C1, phase-1 review): presence in the
	// config source proves nothing about a built container. Grade the whole
	// wiring chain — declared settings carry a real guard envelope, the staged
	// $HOME matches the source, and (below, container tier) the sync happened.
	for _, agent := range sortedAgents(pb.Harness) {
		h := pb.Harness[agent]
		if h.Settings == "" {
			continue
		}
		ok, detail := settingsCarryGuards(resolved.Source, h)
		res.Checks = append(res.Checks, Check{Name: "host:harness:" + agent + ":settings", OK: ok, Detail: detail})
	}
	if len(pb.Dotfiles)+len(pb.Harness) > 0 {
		staging := filepath.Join(resolved.Root, ".loom", "home")
		drift, derr := stagedHomeDrift(resolved.Source, pb, staging)
		switch {
		case derr != nil:
			res.Checks = append(res.Checks, Check{Name: "host:staged-home", OK: false, Detail: derr.Error()})
		case len(drift) > 0:
			res.Checks = append(res.Checks, Check{Name: "host:staged-home", OK: false,
				Detail: fmt.Sprintf("stale vs config source (run build): %v", drift)})
		default:
			res.Checks = append(res.Checks, Check{Name: "host:staged-home", OK: true, Detail: "matches config source"})
		}
	}

	// Git-side guardrails (branch-guard/protect-paths class) reach a repo only
	// through core.hooksPath. A project that ships .githooks but has the config
	// unset runs zero commit-time guards while every hook file sits green.
	if ok, detail, applicable := githooksWired(resolved.Root); applicable {
		res.Checks = append(res.Checks, Check{Name: "host:githooks", OK: ok, Detail: detail})
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

	// Home wiring (C1): the container's home sentinel must match the staged
	// $HOME — guard hooks and settings count only once the sync put them in.
	// Live-readable only (the sentinel lives inside the container); a stopped
	// container is graded at the staging tier above, never started to ask.
	if running {
		if want := homeDigest(filepath.Join(resolved.Root, ".loom", "home")); want != "" {
			have := rt.HomeDigest(cname)
			detail := "home sentinel matches staged config"
			if have != want {
				detail = "container $HOME out of sync with staged config (run build)"
			}
			res.Checks = append(res.Checks, Check{Name: "container:home", OK: have == want, Detail: detail})
		}
	}

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

// settingsCarryGuards parses an agent's declared settings file and verifies it
// carries a real guard envelope: a non-empty permissions.deny floor and a
// registration for every declared harness hook. This is the C1 regression
// check — a settings file that materializes fine but holds only cosmetics
// (statusLine) leaves the built container's agent with zero mechanism-level
// guardrails while every presence check stays green.
func settingsCarryGuards(src fs.FS, h playbook.HarnessAgent) (bool, string) {
	data, err := fs.ReadFile(src, path.Join("dotfiles", h.Settings))
	if err != nil {
		return false, fmt.Sprintf("settings %q unreadable: %v", h.Settings, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return false, fmt.Sprintf("settings %q is not valid JSON: %v", h.Settings, err)
	}

	var perms struct {
		Deny []string `json:"deny"`
	}
	if raw, ok := doc["permissions"]; ok {
		_ = json.Unmarshal(raw, &perms)
	}
	if len(perms.Deny) == 0 {
		return false, "no permissions.deny floor declared"
	}

	hooksRaw, ok := doc["hooks"]
	if !ok && len(h.Hooks) > 0 {
		return false, "declared harness hooks have no hook registrations"
	}
	for _, name := range h.Hooks {
		if !strings.Contains(string(hooksRaw), name) {
			return false, fmt.Sprintf("hook %q declared but not registered in settings", name)
		}
	}
	return true, "deny floor + hook registrations present"
}

// githooksWired grades the git-side guardrail wiring of the project repo:
// applicable only when the project ships a .githooks dir AND is a git repo;
// wired when core.hooksPath points at it.
func githooksWired(root string) (ok bool, detail string, applicable bool) {
	if _, err := os.Stat(filepath.Join(root, ".githooks")); err != nil {
		return false, "", false
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return false, "", false
	}
	out, err := exec.Command("git", "-C", root, "config", "--local", "--get", "core.hooksPath").Output()
	got := strings.TrimSpace(string(out))
	if err != nil || got != ".githooks" {
		return false, ".githooks present but core.hooksPath unset — commit-time guards inert (run build or make hooks)", true
	}
	return true, "core.hooksPath=.githooks", true
}

func sortedAgents(harness map[string]playbook.HarnessAgent) []string {
	agents := make([]string, 0, len(harness))
	for a := range harness {
		agents = append(agents, a)
	}
	sort.Strings(agents)
	return agents
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
