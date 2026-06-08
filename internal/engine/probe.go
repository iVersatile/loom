package engine

import (
	"os/exec"
	"strings"
)

// prober reports whether a tool binary is present and, best-effort, its version.
// It is an interface so detect/plan are hermetically testable with a fake — the
// real implementation shells out, which unit tests must not depend on.
type prober interface {
	probe(binary string) (present bool, version string)
}

// execProber resolves tools via PATH and asks them their version.
type execProber struct{}

func (execProber) probe(binary string) (bool, string) {
	if _, err := exec.LookPath(binary); err != nil {
		return false, ""
	}
	// Tools disagree on the flag (git --version vs go version); try both.
	for _, arg := range []string{"--version", "version"} {
		if out, err := exec.Command(binary, arg).CombinedOutput(); err == nil {
			return true, firstLine(string(out))
		}
	}
	return true, "" // present, version unknown
}

// binaryAlias maps a playbook tool name to its actual binary when they differ.
var binaryAlias = map[string]string{
	"ripgrep": "rg",
}

func binaryName(tool string) string {
	if b, ok := binaryAlias[tool]; ok {
		return b
	}
	return tool
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// versionSatisfies reports whether an installed version satisfies the wanted
// intent. Empty/"latest" is satisfied by presence; otherwise a best-effort
// substring match (Phase 1 heuristic — exact pinning is the resolver's job).
func versionSatisfies(want, installed string) bool {
	if want == "" || want == "latest" {
		return true
	}
	return strings.Contains(installed, want)
}

func wantOrLatest(want string) string {
	if want == "" {
		return "latest"
	}
	return want
}
