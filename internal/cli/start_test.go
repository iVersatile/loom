package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/iVersatile/loom/internal/render"
)

// fakeEngine records the orchestration call order and lets each sub-verb be
// stubbed — start's unit tests drive the full detect→plan→build→exec sequence
// with no docker (the integration test, TestStartOneRunWorkingEnv, exercises
// the real path).
type fakeEngine struct {
	calls []string

	detectRes engine.DetectResult
	detectErr error
	planErr   error
	buildRes  engine.BuildResult
	buildErr  error
	smokeRes  engine.ExecResult
	smokeErr  error
}

func (f *fakeEngine) seam() startEngine {
	return startEngine{
		detect: func(engine.DetectOpts) (engine.DetectResult, error) {
			f.calls = append(f.calls, "detect")
			return f.detectRes, f.detectErr
		},
		plan: func(engine.PlanOpts) (engine.PlanResult, error) {
			f.calls = append(f.calls, "plan")
			return engine.PlanResult{}, f.planErr
		},
		build: func(engine.BuildOpts) (engine.BuildResult, error) {
			f.calls = append(f.calls, "build")
			return f.buildRes, f.buildErr
		},
		smoke: func(engine.ExecOpts) (engine.ExecResult, error) {
			f.calls = append(f.calls, "smoke")
			return f.smokeRes, f.smokeErr
		},
	}
}

// happyEngine is a fake where every sub-verb succeeds and build reports a
// created container — the one-run-to-ready path.
func happyEngine() *fakeEngine {
	return &fakeEngine{
		buildRes: engine.BuildResult{Result: "created", Container: engine.ContainerInfo{Name: "loom-dev"}},
		smokeRes: engine.ExecResult{ExitCode: 0},
	}
}

func nonInteractiveCfg() startConfig {
	return startConfig{
		stack:          "go",
		keyVals:        map[string]string{"ANTHROPIC_API_KEY": "x"},
		nonInteractive: true,
		in:             strings.NewReader(""),
		errOut:         &bytes.Buffer{},
	}
}

// TestStartOrchestratesDetectPlanBuild (FR-RUN-001): start reaches a working
// env by INVOKING the reconcile verbs in order — detect → plan → build (+ the
// smoke exec) — and reports ready:true with the build's container.
func TestStartOrchestratesDetectPlanBuild(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "x") // contain the key pass-through Setenv
	f := happyEngine()
	res, code, err := orchestrateStart(f.seam(), nonInteractiveCfg())
	if err != nil || code != 0 {
		t.Fatalf("happy path: code=%d err=%v, want 0/nil", code, err)
	}
	if want := []string{"detect", "plan", "build", "smoke"}; !slices.Equal(f.calls, want) {
		t.Errorf("call order = %v, want %v (in-order sub-verb invocation)", f.calls, want)
	}
	if !res.Ready {
		t.Error("ready = false, want true after a clean detect→plan→build→exec")
	}
	stepVerbs := make([]string, len(res.Steps))
	for i, s := range res.Steps {
		stepVerbs[i] = s.Verb
	}
	if want := []string{"detect", "plan", "build", "exec"}; !slices.Equal(stepVerbs, want) {
		t.Errorf("steps = %v, want %v", stepVerbs, want)
	}
	if res.Steps[2].Container != "loom-dev" {
		t.Errorf("build step container = %q, want loom-dev", res.Steps[2].Container)
	}
	if !slices.Equal(res.Next, []string{"loom shell"}) {
		t.Errorf("next = %v, want [loom shell]", res.Next)
	}
}

// TestStartAbortsOnSubVerbFailure (FR-RUN-001): a non-zero result from a
// reconcile sub-verb aborts the run with that exit code (1) and ready:false —
// no partial "ready", and no later sub-verb runs.
func TestStartAbortsOnSubVerbFailure(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "x") // contain the key pass-through Setenv
	f := happyEngine()
	f.buildErr = errors.New("container step: docker unavailable")
	res, code, err := orchestrateStart(f.seam(), nonInteractiveCfg())
	if code != 1 || err == nil {
		t.Fatalf("build failure: code=%d err=%v, want 1/non-nil", code, err)
	}
	if res.Ready {
		t.Error("ready = true after a build failure; must be false (no partial ready)")
	}
	if slices.Contains(f.calls, "smoke") {
		t.Errorf("smoke exec ran after build aborted: calls=%v", f.calls)
	}
	last := res.Steps[len(res.Steps)-1]
	if last.Verb != "build" || last.Result != "error" {
		t.Errorf("last step = %+v, want build/error", last)
	}
}

// TestStartNonInteractiveJSONShape (FR-RUN-002): the non-interactive path emits
// EXACTLY the documented --json shape (situation, inputs{stack,keys}, steps[],
// ready, next) — start is not --json-exempt.
func TestStartNonInteractiveJSONShape(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "x") // contain the key pass-through Setenv
	f := happyEngine()
	res, code, err := orchestrateStart(f.seam(), nonInteractiveCfg())
	if code != 0 || err != nil {
		t.Fatalf("orchestrate: code=%d err=%v", code, err)
	}
	var buf bytes.Buffer
	if e := render.Emit(&buf, true, res); e != nil {
		t.Fatalf("emit: %v", e)
	}
	var got map[string]json.RawMessage
	if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", e, buf.String())
	}
	gotKeys := make([]string, 0, len(got))
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	slices.Sort(gotKeys)
	if want := []string{"inputs", "next", "ready", "situation", "steps"}; !slices.Equal(gotKeys, want) {
		t.Errorf("top-level keys = %v, want %v", gotKeys, want)
	}
	var inputs startInputs
	if e := json.Unmarshal(got["inputs"], &inputs); e != nil {
		t.Fatalf("inputs not the documented object: %v", e)
	}
	if inputs.Stack != "go" || !slices.Equal(inputs.Keys, []string{"ANTHROPIC_API_KEY"}) {
		t.Errorf("inputs = %+v, want stack=go keys=[ANTHROPIC_API_KEY]", inputs)
	}
}

// TestStartNonInteractiveMissingInputExit2 (FR-RUN-002): a required input
// missing in non-interactive mode exits 2 (needs-input) — never prompting or
// hanging, and never reaching plan/build.
func TestStartNonInteractiveMissingInputExit2(t *testing.T) {
	// Missing stack.
	t.Run("missing stack", func(t *testing.T) {
		f := happyEngine()
		cfg := nonInteractiveCfg()
		cfg.stack = ""
		res, code, err := orchestrateStart(f.seam(), cfg)
		if code != 2 || err == nil {
			t.Fatalf("missing stack: code=%d err=%v, want 2/non-nil", code, err)
		}
		if res.Ready {
			t.Error("ready = true on needs-input")
		}
		if slices.Contains(f.calls, "plan") || slices.Contains(f.calls, "build") {
			t.Errorf("plan/build ran despite needs-input: calls=%v", f.calls)
		}
	})
	// Missing key (no flag value, env cleared).
	t.Run("missing key", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		f := happyEngine()
		cfg := nonInteractiveCfg()
		cfg.keyVals = map[string]string{}
		_, code, err := orchestrateStart(f.seam(), cfg)
		if code != 2 || err == nil {
			t.Fatalf("missing key: code=%d err=%v, want 2/non-nil", code, err)
		}
		if slices.Contains(f.calls, "build") {
			t.Errorf("build ran despite a missing key: calls=%v", f.calls)
		}
	})
}

// TestStartMinimalInputs (FR-RUN-003): the ONLY inputs start solicits are the
// stack + the required keys. Interactive mode prompts for exactly these (no
// other prompt); non-interactive reads exactly these from flags/env.
func TestStartMinimalInputs(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "") // force the key prompt, not an ambient value

	// Interactive: answer stack then the one key; assert exactly two prompts and
	// that they name the stack and the key — nothing else is asked.
	var errOut bytes.Buffer
	f := happyEngine()
	cfg := startConfig{
		stack:          "",                  // unset → must be prompted
		keyVals:        map[string]string{}, // unset → must be prompted
		nonInteractive: false,
		in:             strings.NewReader("go\nkeyval-xyz\n"),
		errOut:         &errOut,
	}
	res, code, err := orchestrateStart(f.seam(), cfg)
	if code != 0 || err != nil {
		t.Fatalf("interactive minimal inputs: code=%d err=%v", code, err)
	}
	prompts := strings.Count(errOut.String(), ": ")
	if prompts != 2 {
		t.Errorf("emitted %d prompts, want exactly 2 (stack + 1 key); got %q", prompts, errOut.String())
	}
	if !strings.Contains(errOut.String(), "stack") {
		t.Errorf("no stack prompt: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "ANTHROPIC_API_KEY") {
		t.Errorf("no key prompt: %q", errOut.String())
	}
	// The key VALUE never appears in the result (RULES §5: names only).
	blob, _ := json.Marshal(res)
	if strings.Contains(string(blob), "keyval-xyz") {
		t.Errorf("key value leaked into the result: %s", blob)
	}
	if res.Inputs.Stack != "go" || !slices.Equal(res.Inputs.Keys, []string{"ANTHROPIC_API_KEY"}) {
		t.Errorf("inputs = %+v, want stack=go keys=[ANTHROPIC_API_KEY]", res.Inputs)
	}
}

// TestStartNeedsInputExitCodeViaCLI pins the CLI half of FR-RUN-002: the real
// command maps a non-interactive missing input to process exit 2.
func TestStartNeedsInputExitCodeViaCLI(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	// No --stack, no --key, --non-interactive ⇒ needs-input ⇒ exit 2.
	_, _, err := runCmd(t, "start", "--non-interactive", "--json", "-f", fixturePlaybook)
	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("missing input should signal an exit code, got %v", err)
	}
	if ee.code != 2 {
		t.Errorf("needs-input exit code = %d, want 2", ee.code)
	}
}
