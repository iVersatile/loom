package engine

import (
	"encoding/json"
	"testing"
)

func TestPlanChanged(t *testing.T) {
	if (PlanResult{}).Changed() {
		t.Error("empty plan should be converged (not changed)")
	}
	p := PlanResult{Install: []InstallItem{{Tool: "ruff", To: "0.5.2"}}}
	if !p.Changed() {
		t.Error("plan with an install should report changed")
	}
	// noop-only must NOT count as changed — it is the converged signal.
	if (PlanResult{Noop: []string{"go"}}).Changed() {
		t.Error("noop-only plan should be converged")
	}
}

func TestDoctorOK(t *testing.T) {
	if !(DoctorResult{}).OK() {
		t.Error("doctor with no checks should be OK")
	}
	if (DoctorResult{Checks: []Check{{Name: "a", OK: true}, {Name: "b", OK: false}}}).OK() {
		t.Error("doctor with a failing check should not be OK")
	}
}

// The JSON tags ARE the --json contract; lock the keys against accidental rename.
func TestDetectJSONKeys(t *testing.T) {
	b, err := json.Marshal(DetectResult{})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"tools", "agents", "credentials", "projects", "drift"} {
		if _, ok := m[k]; !ok {
			t.Errorf("detect --json missing key %q", k)
		}
	}
}

// Have/From are pointers so "absent" serializes as null, not "".
func TestNullableFields(t *testing.T) {
	b, _ := json.Marshal(Drift{Tool: "ruff", Want: "0.5.2"})
	if got := string(b); got != `{"tool":"ruff","want":"0.5.2","have":null}` {
		t.Errorf("drift JSON = %s, want have:null", got)
	}
}
