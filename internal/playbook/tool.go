package playbook

import "strings"

// SplitTool splits a "name@version" intent into its name and version. A bare
// "name" yields an empty version, meaning latest/unpinned (the resolver pins the
// concrete version into the lockfile, ADR-0002).
func SplitTool(intent string) (name, version string) {
	if i := strings.IndexByte(intent, '@'); i >= 0 {
		return intent[:i], intent[i+1:]
	}
	return intent, ""
}
