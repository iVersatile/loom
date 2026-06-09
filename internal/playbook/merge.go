package playbook

import "slices"

// Merge combines layers in build-time order — base → stack/<lang> →
// overlay/<project> → user-local — into the effective desired-state playbook
// (ADR-0004). Rules:
//   - Scalars (name, stack, overlay, config_source, ...): last non-empty wins.
//   - Lists (tools, rules, dotfiles, hooks, ...): concatenate, then dedup keeping
//     first occurrence. This realizes the frozen rules: resolution — a union of
//     explicit references, with the stack contributing defaults earlier in order.
//
// Note (Phase 1): list dedup is by exact string, so name@version tool entries do
// NOT override by name across layers (both would survive). Name-keyed override is
// a resolver/future concern; the spec says lists concatenate.
func Merge(layers ...*Playbook) *Playbook {
	out := &Playbook{}
	for _, l := range layers {
		if l == nil {
			continue
		}
		if l.Loom != 0 {
			out.Loom = l.Loom
		}
		if l.Tier != "" {
			out.Tier = l.Tier
		}
		if l.Name != "" {
			out.Name = l.Name
		}
		if l.Stack != "" {
			out.Stack = l.Stack
		}
		if l.Overlay != "" {
			out.Overlay = l.Overlay
		}
		if l.Extends != "" {
			out.Extends = l.Extends
		}
		if l.ConfigSource != nil {
			out.ConfigSource = l.ConfigSource
		}
		out.Agents = appendDedup(out.Agents, l.Agents)
		out.Tools = appendDedup(out.Tools, l.Tools)
		out.Rules = appendDedup(out.Rules, l.Rules)
		out.Dotfiles = appendDedup(out.Dotfiles, l.Dotfiles)
		out.Hooks = appendDedup(out.Hooks, l.Hooks)
		out.Ports = appendDedup(out.Ports, l.Ports)
		out.Env = appendDedup(out.Env, l.Env)
		out.CI = appendDedup(out.CI, l.CI)
	}
	return out
}

// appendDedup appends src to dst, skipping values already present (stable order).
func appendDedup[T comparable](dst, src []T) []T {
	for _, v := range src {
		if !slices.Contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

func sortStrings(s []string) { slices.Sort(s) }
