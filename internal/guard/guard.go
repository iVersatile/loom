// Package guard verifies that guardrail hooks are present and executable in the
// config source. Guardrails are mechanism, not trust (ADR-0005, RULES §5), so a
// missing or non-executable hook is a real failure doctor must surface.
package guard

import (
	"io/fs"
	"path"
)

// HookStatus is one guardrail hook's installed state.
type HookStatus struct {
	Name       string
	Present    bool
	Executable bool
}

// OK reports whether the hook is present and executable.
func (h HookStatus) OK() bool { return h.Present && h.Executable }

// Verify checks each named hook under "hooks/" in the config source.
func Verify(src fs.FS, hooks []string) []HookStatus {
	out := make([]HookStatus, 0, len(hooks))
	for _, h := range hooks {
		st := HookStatus{Name: h}
		if info, err := fs.Stat(src, path.Join("hooks", h)); err == nil {
			st.Present = true
			st.Executable = info.Mode().Perm()&0o100 != 0
		}
		out = append(out, st)
	}
	return out
}
