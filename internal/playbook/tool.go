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

// binaryAlias maps a playbook tool name to its actual binary when they differ.
var binaryAlias = map[string]string{
	"ripgrep":     "rg",
	"claude-code": "claude", // the native installer lands the binary as `claude`
}

// BinaryName returns the executable name for a tool name (e.g. ripgrep -> rg).
func BinaryName(tool string) string {
	if b, ok := binaryAlias[tool]; ok {
		return b
	}
	return tool
}

// WantOrLatest renders an empty version intent as "latest".
func WantOrLatest(want string) string {
	if want == "" {
		return "latest"
	}
	return want
}
