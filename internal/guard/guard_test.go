package guard

import (
	"os"
	"testing"
)

func TestVerify(t *testing.T) {
	// The fixture config ships executable hook stubs.
	src := os.DirFS("../playbook/testdata/proj/config")
	got := Verify(src, []string{"guard-bash", "branch-guard", "protect-paths", "does-not-exist"})
	if len(got) != 4 {
		t.Fatalf("want 4 statuses, got %d", len(got))
	}
	for _, h := range got[:3] {
		if !h.OK() {
			t.Errorf("hook %s should be present+executable, got %+v", h.Name, h)
		}
	}
	if got[3].Present {
		t.Errorf("missing hook should report Present=false, got %+v", got[3])
	}
}
