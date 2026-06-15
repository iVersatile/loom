package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iVersatile/loom/internal/playbook"
)

// roleMarkerWired is the mechanized T10 PR-4 reminder. The footgun: the moment a
// playbook configures a NON-ROOT user, the drain-guard's `id -un==root ⇒
// loom-author` fallback stops resolving — on a non-root container the role
// guard fail-closes and the Stop-hook drain silently no-ops, so autonomous
// delivery dies. PR 4 (docs/patches/0022) swaps that guess for the root-owned
// /var/lib/loom/role marker. This check FIRES exactly at the non-root moment if
// the swap hasn't been applied — turning a deferred reminder into a doctor
// claim ("what the reviewer hand-checks once, doctor mechanizes").
//
// Not graded when: user is root/unset (the fallback is correct there) or there
// is no drain hook on the tree (cf. the applicable flag, githooksWired).
func roleMarkerWired(pb playbook.Playbook, root string) (Check, bool) {
	if pb.User == "" || pb.User == "root" {
		return Check{}, false // root container: the id -un==root fallback is correct
	}
	data, err := os.ReadFile(filepath.Join(root, ".claude", "hooks", "drain-inbox.sh"))
	if err != nil {
		return Check{}, false // no drain hook here — nothing to grade
	}
	if strings.Contains(string(data), "/var/lib/loom/role") {
		return Check{Name: "host:role-marker", OK: true, Detail: fmt.Sprintf(
			"non-root user %q: drain-guard reads /var/lib/loom/role (T10 PR 4 applied)", pb.User)}, true
	}
	return Check{Name: "host:role-marker", OK: false, Detail: fmt.Sprintf(
		"non-root user %q but the drain-guard still uses the id -un==root fallback — apply T10 PR 4 "+
			"(docs/patches/0022): on a non-root container the role-guard fail-closes and the Stop-hook "+
			"drain SILENTLY NO-OPS (autonomous delivery stops). Marker source: declare role: in the "+
			"playbook (ADR-0019 PR4 §5) — or LOOM_SESSION_ROLE override — then rebuild", pb.User)}, true
}
