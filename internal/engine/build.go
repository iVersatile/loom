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

// proberVersion adapts the engine prober to resolver.VersionProbe.
type proberVersion struct{ p prober }

func (a proberVersion) Version(binary string) (string, bool) {
	present, v := a.p.probe(binary)
	return v, present
}

// Build materializes the playbook into reality (docs/SPEC-verbs.md "build"):
// resolve intent → write lockfile → materialize $HOME → create the container.
// It is the first mutating verb, so every step appends to the action log.
func Build(opts BuildOpts) (BuildResult, error) {
	return buildImpl(opts, execProber{}, defaultRuntime(), time.Now)
}

func buildImpl(opts BuildOpts, p prober, rt ContainerRuntime, now func() time.Time) (BuildResult, error) {
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

	// 1. Resolve intent → concrete pins.
	resolution := resolver.Resolve(pb, proberVersion{p})
	for name, lt := range resolution.Tools {
		res.Resolved[name] = ResolvedTool{Resolved: lt.Resolved, Source: lt.Source}
	}

	// 2. Lock — rewrite only when the resolved content changes (idempotent).
	lockPath := filepath.Join(root, "loom.lock")
	existing, err := lock.Read(lockPath)
	if err != nil {
		return res, fmt.Errorf("read lock: %w", err)
	}
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

	// Diagnostic log for troubleshooting: raw docker + provision output, written
	// always (success or failure), separate from the structured action log.
	logDir := filepath.Join(root, ".loom", "logs")
	var logw io.Writer
	if mkErr := os.MkdirAll(logDir, 0o755); mkErr == nil {
		if lf, e := os.Create(filepath.Join(logDir, "build.log")); e == nil {
			defer func() { _ = lf.Close() }()
			logw = lf
			res.LogPath = filepath.Join(logDir, "build.log")
			_, _ = fmt.Fprintf(lf, "loom build %s base=%s\n", ts, img)
		}
	}

	// 4. Container — create or converge via the runtime.
	cname := containerName(pb.Name)
	info, err := rt.Ensure(ContainerSpec{
		Name: cname, BaseImage: img, HomeDir: home, Tools: toolInstalls(resolution), Force: opts.Force, LogW: logw,
	})
	if err != nil {
		return res, fmt.Errorf("container step: %w", err)
	}
	res.Container = info
	if info.Status == "created" {
		changed = true
		if id, err := log.Append(audit.Entry{
			TS: ts, Verb: "build", Action: "container.create", Target: cname,
			After: map[string]any{"image": info.Image}, Result: "created", Actor: "cli",
		}); err == nil {
			res.Actions = append(res.Actions, id)
		}
	}

	if changed {
		res.Result = "created"
	}
	return res, nil
}

func toolInstalls(r *resolver.Resolution) []ToolInstall {
	out := make([]ToolInstall, 0, len(r.Tools))
	for name, lt := range r.Tools {
		out = append(out, ToolInstall{Name: name, Source: lt.Source})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
