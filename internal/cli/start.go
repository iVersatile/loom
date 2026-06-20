package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/iVersatile/loom/internal/engine"
	"github.com/spf13/cobra"
)

// start is the guided onboarding entry (docs/SPEC-verbs.md "start", T36 / The
// Run scenario 1): the bring-up verb symmetric with teardown. It ORCHESTRATES
// the existing reconcile verbs — detect → (gather minimal inputs) → plan →
// build → a smoke exec — to take a machine from nothing to a working,
// AI-capable env in ONE run. It adds guidance + UX, never new reconcile logic:
// every sub-verb's own contract + guardrails apply unchanged (FR-RUN-001).
//
// Two interfaces, one verb (charter "three interfaces"): an interactive menu
// for the non-technical human (situation-aware prompts) AND a complete
// non-interactive --json/flag path for the technical human + AI agent. start is
// NOT --json-exempt (unlike exec/shell) — the non-interactive path is the
// e2e-testable surface (FR-RUN-002).

// startRequiredKeys are the credential keys scenario-1 start must have to reach
// an AI-capable env. ANTHROPIC_API_KEY is the canonical one named by the spec;
// scenario 2's credential carry-forward (detect --migrate, ADR-0014/0026) is a
// separate later slice and adds nothing here (FR-RUN-003 scope guard).
var startRequiredKeys = []string{"ANTHROPIC_API_KEY"}

// smokeCommand is the trivial in-container command start runs after build to
// prove the env is enterable + runnable — the "smoke exec" half of the frozen
// one-run success contract (FR-RUN-004). `true` exists on the debian base and
// returns 0; any non-zero/transport failure means the env is not working.
var smokeCommand = []string{"true"}

// startInputs is the inputs object in the --json shape: the ONLY human edits
// scenario 1 takes (FR-RUN-003). keys carries NAMES only — never values
// (RULES §5: no secret values in logs/output).
type startInputs struct {
	Stack string   `json:"stack"`
	Keys  []string `json:"keys"`
}

// startStep is one orchestrated sub-verb outcome in steps[]. Container is set
// only by build (the created/converged container), matching the spec shape.
type startStep struct {
	Verb      string `json:"verb"`
	Result    string `json:"result"`
	Container string `json:"container,omitempty"`
}

// startResult is the --json shape for start (docs/SPEC-verbs.md "start"):
// situation, inputs, steps[], ready, next. Top-level keys are frozen — the
// conformance test (TestSpecConformance) extracts them from the spec block and
// compares against this struct's tags, so no omitempty on the top level.
type startResult struct {
	Situation string      `json:"situation"`
	Inputs    startInputs `json:"inputs"`
	Steps     []startStep `json:"steps"`
	Ready     bool        `json:"ready"`
	Next      []string    `json:"next"`
}

func (r startResult) Human() string {
	status := "not ready"
	if r.Ready {
		status = "ready"
	}
	return fmt.Sprintf("start: %s (situation: %s) — ran %d step(s)", status, r.Situation, len(r.Steps))
}

// startEngine is the orchestration seam: start calls these instead of the
// engine package directly so the unit tests can drive the full sequence with
// fakes (no docker). liveStartEngine wires the real entry points — start NEVER
// reimplements their logic (FR-RUN-001).
type startEngine struct {
	detect func(engine.DetectOpts) (engine.DetectResult, error)
	plan   func(engine.PlanOpts) (engine.PlanResult, error)
	build  func(engine.BuildOpts) (engine.BuildResult, error)
	smoke  func(engine.ExecOpts) (engine.ExecResult, error)
}

func liveStartEngine() startEngine {
	return startEngine{
		detect: engine.Detect,
		plan:   engine.Plan,
		build:  engine.Build,
		smoke:  engine.Exec,
	}
}

// startConfig is the resolved invocation (flags already parsed). keyVals holds
// values supplied via --key NAME=VALUE; a bare --key NAME maps to "" and is
// resolved from the process env. in/errOut drive interactive prompting.
type startConfig struct {
	playbook       string
	stack          string
	keyVals        map[string]string
	nonInteractive bool
	in             io.Reader
	errOut         io.Writer
}

// orchestrateStart sequences detect → gather inputs → plan → build → smoke exec
// and returns the result, a process exit code (0 ready / 1 error / 2
// needs-input), and a non-nil err describing an abort (printed by the caller).
//
//   - A non-zero result from any reconcile sub-verb aborts the run with exit 1
//     and ready:false — no partial "ready" (FR-RUN-001). plan reporting
//     "changes needed" is NOT a failure here: those changes are precisely what a
//     fresh-machine bring-up then builds, so only a genuine sub-verb ERROR
//     aborts (clause-ambiguity resolution — see FINDINGS).
//   - A required input missing in non-interactive mode exits 2 (needs-input),
//     never prompting or hanging (FR-RUN-002).
//   - ready:true ONLY when build did not error AND the smoke exec succeeds
//     (FR-RUN-004).
func orchestrateStart(eng startEngine, cfg startConfig) (startResult, int, error) {
	res := startResult{Steps: []startStep{}, Next: []string{}}
	reader := bufio.NewReader(cfg.in)

	// 1. detect — read the situation (never mutates). A failure aborts.
	dres, derr := eng.detect(engine.DetectOpts{PlaybookPath: cfg.playbook})
	if derr != nil {
		res.Steps = append(res.Steps, startStep{Verb: "detect", Result: "error"})
		return res, 1, fmt.Errorf("detect: %w", derr)
	}
	res.Situation = situationFrom(dres)
	res.Steps = append(res.Steps, startStep{Verb: "detect", Result: "ok"})

	// 2. gather the minimal inputs — stack + keys, nothing else (FR-RUN-003).
	//    Interactive ⇒ prompted; non-interactive ⇒ flags/env, missing ⇒ exit 2.
	stack := cfg.stack
	if stack == "" {
		if cfg.nonInteractive {
			return res, 2, fmt.Errorf("missing required input: --stack (non-interactive)")
		}
		stack = promptLine(reader, cfg.errOut, "stack (e.g. go, python): ")
		if stack == "" {
			return res, 2, fmt.Errorf("missing required input: stack")
		}
	}
	res.Inputs.Stack = stack

	keyNames := append([]string{}, startRequiredKeys...)
	sort.Strings(keyNames)
	for _, name := range keyNames {
		val, supplied := cfg.keyVals[name]
		if !supplied || val == "" {
			// A value already in the process env satisfies the key and is
			// carried by the existing build env path (docker run -e NAME).
			if os.Getenv(name) != "" {
				continue
			}
			if cfg.nonInteractive {
				return res, 2, fmt.Errorf("missing required key %s (pass --key %s=… or export it)", name, name)
			}
			val = promptLine(reader, cfg.errOut, fmt.Sprintf("%s: ", name))
			if val == "" {
				return res, 2, fmt.Errorf("missing required key %s", name)
			}
		}
		// Pass-through into the existing build env path: export the value so
		// build's `docker run -e NAME` (for a name the playbook declares)
		// carries it into the container. Values are never logged or emitted.
		_ = os.Setenv(name, val)
	}
	res.Inputs.Keys = keyNames

	// 3. plan — preview the diff (never mutates). On a fresh machine plan reports
	//    changes needed; that is the premise of bring-up, not an abort. Only a
	//    genuine plan ERROR aborts the run.
	if _, perr := eng.plan(engine.PlanOpts{PlaybookPath: cfg.playbook}); perr != nil {
		res.Steps = append(res.Steps, startStep{Verb: "plan", Result: "error"})
		return res, 1, fmt.Errorf("plan: %w", perr)
	}
	res.Steps = append(res.Steps, startStep{Verb: "plan", Result: "ok"})

	// 4. build — materialize the env. A failure aborts with ready:false.
	bres, berr := eng.build(engine.BuildOpts{PlaybookPath: cfg.playbook})
	if berr != nil {
		res.Steps = append(res.Steps, startStep{Verb: "build", Result: "error"})
		return res, 1, fmt.Errorf("build: %w", berr)
	}
	res.Steps = append(res.Steps, startStep{Verb: "build", Result: bres.Result, Container: bres.Container.Name})

	// 5. smoke exec — prove the env is enterable + runnable. ready:true is gated
	//    on this succeeding, so "ready" always means a genuinely working env
	//    (FR-RUN-004). A non-zero exit or transport error is not-ready, exit 1.
	exr, eerr := eng.smoke(engine.ExecOpts{PlaybookPath: cfg.playbook, Command: smokeCommand})
	if eerr != nil || exr.ExitCode != 0 {
		res.Steps = append(res.Steps, startStep{Verb: "exec", Result: "error"})
		if eerr != nil {
			return res, 1, fmt.Errorf("smoke exec: %w", eerr)
		}
		return res, 1, fmt.Errorf("smoke exec exited %d (env not working)", exr.ExitCode)
	}
	res.Steps = append(res.Steps, startStep{Verb: "exec", Result: "ok"})

	res.Ready = true
	res.Next = []string{"loom shell"}
	return res, 0, nil
}

// situationFrom derives the situation label from detect's reality (scenario-1
// lens): a machine with any declared tool or agent already present reads as
// established; an otherwise-bare machine is fresh. Descriptive only — it never
// gates readiness.
func situationFrom(d engine.DetectResult) string {
	for _, t := range d.Tools {
		if t.Present {
			return "established-machine"
		}
	}
	for _, a := range d.Agents {
		if a.Present {
			return "established-machine"
		}
	}
	return "fresh-machine"
}

// promptLine writes a prompt to out and reads one trimmed line from the shared
// reader. The interactive menu is deliberately THIN (prompt wrappers); the
// non-interactive path is the testable surface (build spec §1).
func promptLine(reader *bufio.Reader, out io.Writer, prompt string) string {
	_, _ = fmt.Fprint(out, prompt)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// parseKeyFlags turns repeated --key NAME=VALUE flags into a name→value map. A
// bare --key NAME (no '=') maps to "" — the value is then resolved from the
// process env. An empty name is a usage error.
func parseKeyFlags(raw []string) (map[string]string, error) {
	out := map[string]string{}
	for _, kv := range raw {
		name, val, _ := strings.Cut(kv, "=")
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("invalid --key %q: empty key name", kv)
		}
		out[name] = val
	}
	return out, nil
}

func newStartCmd() *cobra.Command { return startCmd(liveStartEngine()) }

// startCmd builds the start command around an orchestration seam (live in
// production; fakeable in tests).
func startCmd(eng startEngine) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Guided bring-up: detect → inputs → plan → build → ready (the one guided run)",
		Long: `Brings a machine from nothing to a working, AI-capable env in a single pass —
the bring-up verb, symmetric with teardown. start ORCHESTRATES the existing
reconcile verbs (detect → plan → build) plus a smoke exec; it adds guidance,
not new reconcile logic.

Two interfaces, one verb: an interactive menu (situation-aware prompts for the
stack + required keys) and a complete non-interactive path —
'loom start --non-interactive --json --stack <stack> --key NAME=VALUE' runs to
completion with zero prompts. A required input missing in --non-interactive
exits 2 (needs-input), never hanging. Exit codes: 0 ready / 1 error / 2
needs-input.`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().Bool("non-interactive", false, "run with zero prompts; a missing required input exits 2")
	cmd.Flags().String("stack", "", "the stack to bring up (required input)")
	cmd.Flags().StringArray("key", nil, "a required credential as NAME=VALUE (repeatable); value pass-through, never logged")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
		stack, _ := cmd.Flags().GetString("stack")
		rawKeys, _ := cmd.Flags().GetStringArray("key")
		keyVals, err := parseKeyFlags(rawKeys)
		if err != nil {
			return err
		}
		res, code, aerr := orchestrateStart(eng, startConfig{
			playbook:       playbookPath(cmd),
			stack:          stack,
			keyVals:        keyVals,
			nonInteractive: nonInteractive,
			in:             cmd.InOrStdin(),
			errOut:         cmd.ErrOrStderr(),
		})
		// start is NOT --json-exempt: the result is always emitted (the
		// documented shape), then the exit code/abort message follows.
		if e := emit(cmd, res, nil); e != nil {
			return e
		}
		if aerr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loom: start: %v\n", aerr)
		}
		switch code {
		case 0:
			return nil
		case 2:
			return &exitError{code: 2}
		default:
			return &exitError{code: 1}
		}
	}
	return cmd
}
