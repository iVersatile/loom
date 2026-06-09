// Package resolver maps thin playbook intent (name@version) to concrete pins
// (resolved version + source + digest), producing a loom.lock (ADR-0002). The
// engine owns this intent→concrete layer; the playbook never carries exact pins.
//
// Phase 1 scope: a small, known tool set for the Go dogfood. Sources follow a
// static policy (go from the base image, gopls via go install, uv via its
// installer, everything else apt on debian:bookworm-slim). Exact digests are
// pinned when build resolves against the pulled image (later); Phase 1 records
// the resolved version observed via a VersionProbe.
package resolver

import (
	"github.com/iVersatile/loom/internal/lock"
	"github.com/iVersatile/loom/internal/playbook"
)

// VersionProbe resolves the concrete version of a tool's binary. Injected so the
// resolver is testable without shelling out.
type VersionProbe interface {
	Version(binary string) (resolved string, found bool)
}

// sourcePolicy overrides the default apt source for tools installed otherwise.
// debian:bookworm-slim ships neither a current Go nor gitleaks, so those are not
// apt: Go comes from the official tarball, gitleaks/gopls via `go install`.
var sourcePolicy = map[string]string{
	"go":       "go-tarball",
	"gopls":    "go-install",
	"gitleaks": "go-install",
	"uv":       "uv-installer",
}

func sourceFor(tool string) string {
	if s, ok := sourcePolicy[tool]; ok {
		return s
	}
	return "apt" // default install source on the debian base
}

// Resolution is the resolved tool/agent set, ready to become a Lock.
type Resolution struct {
	Tools  map[string]lock.LockedTool
	Agents map[string]lock.LockedAgent
}

// Resolve turns the merged playbook's intents into a Resolution using vp for
// concrete versions.
func Resolve(pb *playbook.Playbook, vp VersionProbe) *Resolution {
	r := &Resolution{
		Tools:  map[string]lock.LockedTool{},
		Agents: map[string]lock.LockedAgent{},
	}
	for _, intent := range pb.Tools {
		name, want := playbook.SplitTool(intent)
		resolved, _ := vp.Version(playbook.BinaryName(name))
		r.Tools[name] = lock.LockedTool{
			Intent:   playbook.WantOrLatest(want),
			Resolved: resolved,
			Source:   sourceFor(name),
		}
	}
	for _, a := range pb.Agents {
		resolved, _ := vp.Version(playbook.BinaryName(a))
		r.Agents[a] = lock.LockedAgent{Resolved: resolved}
	}
	return r
}

// Lock assembles the lockfile document from the resolution.
func (r *Resolution) Lock(baseImage, resolvedAt string) *lock.Lock {
	return &lock.Lock{
		LoomLock:   lock.Version,
		ResolvedAt: resolvedAt,
		BaseImage:  baseImage,
		Tools:      r.Tools,
		Agents:     r.Agents,
	}
}
