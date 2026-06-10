package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	for _, m := range mats {
		res.Materialized = append(res.Materialized, m.Display)
		if !m.Changed {
			continue
		}
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "materialize", Target: m.Display,
			Result: "written", Actor: "cli",
		}); err == nil {
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
