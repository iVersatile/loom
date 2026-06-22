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
	"github.com/iVersatile/loom/internal/resolver"
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

	// Declared git identity (T16 PR 3): a playbook that ships `gitconfig`
	// must declare a COMPLETE [user] identity — the materialized ~/.gitconfig
	// is what every in-container commit signs as (TEAM.md commit-identity
	// rule; ADR-0015 declared-config side). Doctor verifies completeness
	// mechanically and surfaces the value; WHICH identity it is stays repo
	// policy, reviewed at the config-source PR (human acceptance via merge).
	for _, ref := range pb.Dotfiles {
		if ref != "gitconfig" {
			continue
		}
		data, rerr := fs.ReadFile(resolved.Source, "dotfiles/"+ref)
		if rerr != nil {
			res.Checks = append(res.Checks, Check{Name: "host:gitconfig", OK: false,
				Detail: fmt.Sprintf("declared gitconfig unreadable: %v", rerr)})
			continue
		}
		name, email := gitIdentity(data)
		if name == "" || email == "" {
			res.Checks = append(res.Checks, Check{Name: "host:gitconfig", OK: false,
				Detail: "missing user.name/user.email — in-container commits would sign as the image default"})
			continue
		}
		res.Checks = append(res.Checks, Check{Name: "host:gitconfig", OK: true,
			Detail: fmt.Sprintf("declares %s <%s>", name, email)})
	}

	// Same expected set build writes and plan grades (F2/C1 doctrine):
	// declared dotfiles + harness + the engine-generated PATH dotfiles (T4).
	resolution := resolver.Resolve(pb, nullVersions{})
	staging := filepath.Join(resolved.Root, ".loom", "home")
	generated := expectedPathDotfiles(toolInstalls(resolution), agentInstalls(resolution), staging)
	if len(pb.Dotfiles)+len(pb.Harness)+len(generated) > 0 {
		drift, derr := stagedHomeDrift(resolved.Source, pb, generated, staging)
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

	// AUTOPILOT drain liveness (T27 facet B; human drafts 032/033). Mechanizes
	// the hand-check the 2026-06-12 silent-idle incident needed: autopilot on
	// with eligible QUEUED work but no drain evidence = anomaly.
	if c, ok := autopilotDrainLiveness(resolved.Root); ok {
		res.Checks = append(res.Checks, c)
	}

	// Role-marker reminder (T10 PR 4): fires the moment a non-root user: is
	// configured but the drain-guard still uses the id -un==root fallback —
	// which would fail-close and silence the drain on a non-root container.
	if c, ok := roleMarkerWired(*pb, resolved.Root); ok {
		res.Checks = append(res.Checks, c)
	}

	// Credential slug-uniqueness (ADR-0027 invariant #1, mechanically checked):
	// the per-project credential volume key isolates one project's credential from
	// another's; this fails closed on a slug that is malformed or could collide
	// (cross-wiring two projects onto one credential store). Host-side + static;
	// graded only when an M1 (volume-token) credential is declared.
	if c, ok := credentialSlugChecks(*pb); ok {
		res.Checks = append(res.Checks, c)
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

	// container:user surfaces the container's runtime-user model (ADR-0019 PR4
	// Part 1) — the companion to the role marker. Under Model A the container
	// always RUNS as root; a configured non-root user is applied per entry-verb
	// via `exec -u <user>`. Making this visible is what the reviewer hand-checked
	// once; doctor mechanizes it, so the operator sees the runtime user that the
	// drain-guard's role resolution depends on. Derived statically from the merged
	// playbook (the Model A invariant guarantees the runtime user is root).
	userModel := "root (Model A: container runs as root)"
	if pb.User != "" && pb.User != "root" {
		userModel = fmt.Sprintf("root (Model A: container runs as root; entry verbs exec -u %s)", pb.User)
	}
	res.Checks = append(res.Checks, Check{Name: "container:user", OK: true, Detail: userModel})

	// container:egress (T20 S2a/S2b, ADR-0028 + Amendment 1): the declared
	// networking posture must match the realized container. egress: none ⇒ the
	// container must have only loopback (the --network none cut, S2a); allowlist ⇒
	// the container must be on its internal egress network and NOT on a bridge with
	// a route out, with its proxy sidecar running (S2b); off/unset ⇒ a no-op pass
	// (full egress is the Phase-1 default, nothing to verify). The off/unset pass is
	// static (no probe); the none/allowlist verifications read live state, so they
	// are gated on a running container (doctor never Starts one to ask — a stopped
	// container with a none/allowlist posture is graded by container:state above,
	// not invented here).
	egressNeedsProbe := pb.Networking != nil &&
		(pb.Networking.Egress == playbook.EgressNone || pb.Networking.Egress == playbook.EgressAllowlist)
	if !egressNeedsProbe {
		res.Checks = append(res.Checks, containerEgressCheck(rt, cname, *pb))
	} else if running {
		res.Checks = append(res.Checks, containerEgressCheck(rt, cname, *pb))
	}

	// Non-root verification (T10 PR4 Part 3, adv-064): mechanize the ad-hoc exec
	// spot-checks that proved the non-root seat into durable doctor claims — the
	// marker is non-forgeable by the runtime user, and that user can write the
	// workspace (the loop-survival invariant). Live-only (read-only docker probes;
	// doctor never Starts a container) and graded only for a non-root user.
	if running {
		res.Checks = append(res.Checks, nonRootContainerChecks(rt, cname, *pb)...)
	}

	// oauth-file credential presence (ADR-0027 slice 2, fail-closed): a declared
	// oauth-file credential's per-project volume must hold a PRESENT + NON-EMPTY creds
	// file at its HOME path (the slice-1 empty-token guard, mirrored at verify time).
	// The probe reads existence/non-empty ONLY (never the body); a missing/empty file
	// FAILS CLOSED. Live-only (in-container probe; doctor never Starts a container) and
	// graded only when an oauth-file credential is declared.
	if running {
		if checks, ok := oauthFileCredentialChecks(rt, cname, homeForUser(pb.User), *pb); ok {
			res.Checks = append(res.Checks, checks...)
		}
	}

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

// gitIdentity scans a gitconfig for the [user] section's name and email — a
// minimal INI walk (sections + key=value), no dependency; enough to verify the
// declared identity is complete. Keys outside [user] are ignored.
func gitIdentity(data []byte) (name, email string) {
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "["):
			section = strings.Trim(line, "[]")
		case section == "user":
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch strings.TrimSpace(k) {
			case "name":
				name = strings.TrimSpace(v)
			case "email":
				email = strings.TrimSpace(v)
			}
		}
	}
	return name, email
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
