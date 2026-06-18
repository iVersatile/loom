package engine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/iVersatile/loom/internal/audit"
	"github.com/iVersatile/loom/internal/lock"
	"github.com/iVersatile/loom/internal/playbook"
	"github.com/iVersatile/loom/internal/resolver"
)

// defaultBaseImage is the shared floor every project container builds on
// (ADR-0001). Overridable via LOOM_BASE_IMAGE so CI can point at the ghcr
// mirror (avoiding Docker Hub rate limits) while local stays on Docker Hub.
const defaultBaseImage = "debian:bookworm-slim"

// baseImage returns the configured base image reference.
func baseImage() string {
	if v := os.Getenv("LOOM_BASE_IMAGE"); v != "" {
		return v
	}
	return defaultBaseImage
}

// sessionRole resolves the container's loom-role identity for the
// /var/lib/loom/role marker (ADR-0019 PR4 §5, LL-014). The DECLARATIVE source is
// the playbook `role:` field — tree-recorded, so `loom build` reproduces the
// marker identically on every host (the LL-014 fix: no host-only hand-write).
// LOOM_SESSION_ROLE is a DEMOTED explicit override (it wins when set, so a second
// seat sharing one tree — e.g. the advisor — overrides without editing the
// playbook) / test-seam. Empty ⇒ no marker; a malformed value is rejected
// downstream by validRole (no marker, fail-safe).
func sessionRole(pb *playbook.Playbook) string {
	if v := strings.TrimSpace(os.Getenv("LOOM_SESSION_ROLE")); v != "" {
		return v
	}
	return strings.TrimSpace(pb.Role)
}

// containerVersions adapts the runtime's in-container probe to
// resolver.VersionProbe: the lock pins the CONTAINER's reality (T5), never the
// build host's — a Mac build must not record "Apple Git" for a debian container.
type containerVersions struct {
	rt   ContainerRuntime
	name string
}

func (c containerVersions) Version(binary string) (string, bool) {
	present, v := c.rt.Probe(c.name, binary)
	return v, present
}

// nullVersions resolves nothing: used for the pre-container resolve pass, where
// no truthful `resolved` source exists yet (the container is the only machine
// whose versions the lock may record, T5).
type nullVersions struct{}

func (nullVersions) Version(string) (string, bool) { return "", false }

// Build materializes the playbook into reality (docs/SPEC-verbs.md "build"):
// resolve intent → write lockfile → materialize $HOME → create the container.
// It is the first mutating verb, so every step appends to the action log.
func Build(opts BuildOpts) (BuildResult, error) {
	return buildImpl(opts, defaultRuntime(), time.Now)
}

func buildImpl(opts BuildOpts, rt ContainerRuntime, now func() time.Time) (BuildResult, error) {
	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}
	resolved, err := playbook.Load(path)
	if err != nil {
		return BuildResult{}, fmt.Errorf("build requires a playbook (%s): %w", path, err)
	}
	pb := resolved.Playbook
	root := resolved.Root
	ts := now().UTC().Format(time.RFC3339)

	log, err := audit.Open(root)
	if err != nil {
		return BuildResult{}, fmt.Errorf("open action log: %w", err)
	}

	res := BuildResult{
		Resolved:     map[string]ResolvedTool{},
		Materialized: []string{},
		Actions:      []string{},
		Result:       "converged",
	}
	changed := false

	// Role marker plan (LL-014 defect 2; keyed on (user, role) per the frozen SPEC
	// + ADR-0019 §5, adv-067 TASK 2). Fail LOUD, never the old silent no-op: a
	// non-root user with no role: is a HARD error (an empty marker silently breaks
	// the drain role-guard on a non-root container); root + no role stays a visible
	// warning (root build byte-identical); a malformed role is a typo to fix.
	// Checked before the lock/home writes so a hard error fails fast, no side effects.
	role := sessionRole(pb)
	roleWarn, roleErr := roleMarkerPlan(pb.User, role)
	if roleErr != nil {
		return res, fmt.Errorf("build: %w", roleErr)
	}
	if roleWarn != "" {
		res.Warnings = append(res.Warnings, roleWarn)
	}

	// 1. Resolve intent → concrete pins. `resolved` versions are probed inside
	// the container AFTER it converges (step 5, T5); this pass carries forward
	// the prior lock's container-probed values (same intent+source) so an
	// unchanged setup stays a no-op, and leaves new tools "" — honest until the
	// container can answer. Never the host PATH.
	lockPath := filepath.Join(root, "loom.lock")
	existing, err := lock.Read(lockPath)
	if err != nil {
		return res, fmt.Errorf("read lock: %w", err)
	}
	resolution := resolver.Resolve(pb, nullVersions{})
	carryForwardResolved(resolution, existing)
	for name, lt := range resolution.Tools {
		res.Resolved[name] = ResolvedTool{Resolved: lt.Resolved, Source: lt.Source}
	}

	// 2. Lock — rewrite only when the resolved content changes (idempotent).
	// Pin the base image to its manifest-list digest when a daemon can resolve
	// it (reproducible across arches); fall back to the plain tag otherwise.
	img := baseImage()
	if digest, derr := rt.ResolveBaseDigest(img); derr == nil && digest != "" {
		img = img + "@" + digest
	}
	newLock := resolution.Lock(img, ts)
	if lock.ContentEqual(existing, newLock) {
		res.LockWritten = false
	} else {
		if err := newLock.WriteFile(lockPath); err != nil {
			return res, fmt.Errorf("write lock: %w", err)
		}
		res.LockWritten = true
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "lock.write", Target: "loom.lock",
			After: map[string]any{"base_image": newLock.BaseImage}, Result: "written", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	}

	// 3. Materialize $HOME into a host staging dir that seeds the container home.
	//    Host-side so it is verifiable without a daemon and survives rebuild.
	home := filepath.Join(root, ".loom", "home")
	mats, err := materializeDotfiles(resolved.Source, pb.Dotfiles, home)
	if err != nil {
		return res, fmt.Errorf("materialize: %w", err)
	}
	// harness: artifacts ride the same staging dir, so the T7 home-digest
	// sentinel covers them — a harness-only change re-syncs the container
	// home exactly like a dotfile change (SPEC-playbook#harness, ADR-0015).
	hmats, err := materializeHarness(resolved.Source, pb.Harness, home)
	if err != nil {
		return res, fmt.Errorf("materialize harness: %w", err)
	}
	mats = append(mats, hmats...)
	// T4: engine-generated PATH dotfiles ride the same staging dir — one
	// dotfile dir owns shell config; the provision script no longer appends
	// PATH lines to shell-init files.
	pmats, err := materializeFiles(expectedPathDotfiles(toolInstalls(resolution), agentInstalls(resolution), home))
	if err != nil {
		return res, fmt.Errorf("materialize path dotfiles: %w", err)
	}
	mats = append(mats, pmats...)
	for _, m := range mats {
		res.Materialized = append(res.Materialized, m.Display)
		if !m.Changed {
			continue
		}
		res.Written++
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "materialize", Target: m.Display,
			Result: "written", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	}

	// Git-side guardrail wiring (C1, phase-1 review): a project that ships
	// .githooks gets core.hooksPath converged so the commit-time guards
	// (branch-guard/protect-paths class) actually run — a declared guard no
	// commit path invokes is fiction. Idempotent; audited like every mutation.
	// NON-FATAL: hooks wiring is a host-side convenience, not load-bearing for
	// the container build (which step 4 produces). A git quirk here — a
	// permission/topology failure on the config write — must not abort a build
	// whose lock/home already converged and whose container has not yet been
	// created; surface it as a warning and press on (FR-BUILD-005 recoverable).
	if wired, werr := ensureGithooksPath(root); werr != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("githooks wiring skipped (commit-time guards may be inert): %v", werr))
	} else if wired {
		changed = true
		if id, aerr := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "githooks.wire", Target: root,
			After: map[string]any{"core.hooksPath": ".githooks"}, Result: "written", Actor: "cli",
		}); aerr == nil {
			res.Actions = append(res.Actions, id)
		}
	}

	// Diagnostic log for troubleshooting (ADR-0010): raw docker + provision output,
	// written always, separate from the structured action log.
	var logw io.Writer
	if lf, path := openDiagLog(root, "build"); lf != nil {
		defer func() { _ = lf.Close() }()
		logw = lf
		res.LogPath = path
		_, _ = fmt.Fprintf(lf, "loom build %s base=%s\n", ts, img)
		for _, w := range res.Warnings {
			_, _ = fmt.Fprintf(lf, "loom: WARNING %s\n", w)
		}
	}

	// 4. Container — create or converge via the runtime. The project root is
	// bind-mounted (T13), so it must be absolute for `docker run -v`.
	cname := containerName(pb.Name)
	projDir := root
	if abs, aerr := filepath.Abs(root); aerr == nil {
		projDir = abs
	}
	info, err := rt.Ensure(ContainerSpec{
		Name: cname, Project: pb.Name, BaseImage: img, HomeDir: home,
		Tools:      toolInstalls(resolution),
		Agents:     agentInstalls(resolution),
		Env:        pb.Env,
		ProjectDir: projDir,
		User:       pb.User,              // T10/ADR-0019: configured runtime user ("" = root)
		Home:       homeForUser(pb.User), // resolved $HOME; consumed by the engine in PR 3
		Role:       role,                 // ADR-0019 PR4 §5: declarative role: → /var/lib/loom/role marker
		Force:      opts.Force, LogW: logw,
	})
	if err != nil {
		return res, fmt.Errorf("container step: %w", err)
	}
	res.Container = info
	switch info.Status {
	case "created":
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "container.create", Target: cname,
			After: map[string]any{"image": info.Image}, Result: "created", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	case "converged":
		// Container existed but was under-provisioned (a prior build interrupted
		// mid-provision, ADR-0011) or drifted — toolset or staged $HOME content
		// (T7); the runtime re-converged it to the declared state. That is a
		// mutation, so audit it (RULES §5).
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "container.reconcile", Target: cname,
			After: map[string]any{"image": info.Image}, Result: "converged", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	}

	// Home-sync audit completeness (live-build e2e F-b): the bulk `docker cp`
	// ships EVERY staged file into the container — including files this run
	// did not rewrite on the host, which the per-write materialize entries
	// above cannot account for. One entry names the full shipped set,
	// reconcilable against `materialized` and the home digest.
	if info.HomeSynced {
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "home.sync", Target: cname,
			After:  map[string]any{"files": res.Materialized, "digest": homeDigest(home)},
			Result: "synced", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	}

	// 5. Re-pin `resolved` from inside the converged container (T5) and rewrite
	// the lock when reality differs. A binary the container lacks stays "" —
	// honest — never a host value. Runs only after a successful container step,
	// so a failed build keeps the carried-forward lock (recoverable,
	// FR-BUILD-005).
	reprobed := resolver.Resolve(pb, containerVersions{rt, cname})
	finalLock := reprobed.Lock(img, ts)
	if current, rerr := lock.Read(lockPath); rerr == nil && !lock.ContentEqual(current, finalLock) {
		if err := finalLock.WriteFile(lockPath); err != nil {
			return res, fmt.Errorf("write lock (container re-pin): %w", err)
		}
		res.LockWritten = true
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "lock.write", Target: "loom.lock",
			After: map[string]any{"base_image": finalLock.BaseImage, "repinned": "container"}, Result: "written", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	}
	for name, lt := range reprobed.Tools {
		res.Resolved[name] = ResolvedTool{Resolved: lt.Resolved, Source: lt.Source}
	}

	// Result enum (SPEC-verbs): a newly created container is "created"; any other
	// convergence (lock / materialize / reconcile) is "converged" (the default).
	if info.Status == "created" {
		res.Result = "created"
	} else if changed {
		res.Result = "converged"
	}
	return res, nil
}

// ensureGithooksPath sets repo-local core.hooksPath=.githooks when the project
// ships a .githooks dir and the config does not already point there. Returns
// true only when it wrote the config (idempotent: a wired repo is a no-op).
// Not applicable (no .githooks, not a git repo, a linked worktree) is a silent
// skip, not an error.
func ensureGithooksPath(root string) (bool, error) {
	if _, err := os.Stat(filepath.Join(root, ".githooks")); err != nil {
		return false, nil
	}
	gi, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		return false, nil
	}
	// A linked worktree has a `.git` FILE (a `gitdir:` pointer), not a dir
	// (LL-016 class — `loom build --playbook <worktree>/loom.yml`). `git config
	// --local` from a worktree writes the SHARED common config (.git/config of
	// the main checkout): wiring here would clobber the main checkout's
	// core.hooksPath as a side effect of building a worktree, and the gitdir
	// pointer may be unresolvable from this host (exit 128). The worktree
	// already inherits hooks via that shared config — wiring is the main
	// checkout's job, so building from a worktree is a no-op for hooks.
	if !gi.IsDir() {
		return false, nil
	}
	out, _ := exec.Command("git", "-C", root, "config", "--local", "--get", "core.hooksPath").Output()
	if strings.TrimSpace(string(out)) == ".githooks" {
		return false, nil
	}
	if err := exec.Command("git", "-C", root, "config", "--local", "core.hooksPath", ".githooks").Run(); err != nil {
		return false, fmt.Errorf("git config core.hooksPath: %w", err)
	}
	return true, nil
}

// carryForwardResolved seeds a fresh resolution's `resolved` fields from the
// existing lock when the tool's intent+source are unchanged — those values were
// container-probed by a prior build (T5), so they remain the best truth until
// the container can be re-probed. Anything new or changed stays "".
func carryForwardResolved(r *resolver.Resolution, existing *lock.Lock) {
	if existing == nil {
		return
	}
	for name, lt := range r.Tools {
		if old, ok := existing.Tools[name]; ok && old.Intent == lt.Intent && old.Source == lt.Source {
			lt.Resolved = old.Resolved
			r.Tools[name] = lt
		}
	}
	for name, la := range r.Agents {
		if old, ok := existing.Agents[name]; ok {
			la.Resolved = old.Resolved
			r.Agents[name] = la
		}
	}
}

func toolInstalls(r *resolver.Resolution) []ToolInstall {
	out := make([]ToolInstall, 0, len(r.Tools))
	for name, lt := range r.Tools {
		out = append(out, ToolInstall{Name: name, Source: lt.Source})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// agentInstalls turns the resolved agent set into install specs for the container
// (T8). Mirrors toolInstalls; sorted for a deterministic provision/digest.
func agentInstalls(r *resolver.Resolution) []AgentInstall {
	out := make([]AgentInstall, 0, len(r.Agents))
	for name := range r.Agents {
		out = append(out, AgentInstall{Name: name, Source: agentSource(name)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// agentSource picks an agent's install mechanism. Phase 1: claude-code ships a
// native installer; other declared agents are recorded but have no installer yet.
func agentSource(name string) string {
	switch name {
	case "claude-code":
		return "native-installer"
	default:
		return ""
	}
}
