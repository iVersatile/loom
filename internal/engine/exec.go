// exec — run one command inside the project container (docs/SPEC-verbs.md
// "exec", ADR-0016). Transparent passthrough: stdout/stderr/exit belong to the
// executed command; the structured record is the audit entry (command, exit
// code, entry id). Doors, not checkpoints: no filtering happens here — the
// container's own guard envelope is the enforcement layer.
package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/iVersatile/loom/internal/audit"
	"github.com/iVersatile/loom/internal/playbook"
)

// ExecResult reports the executed command's exit code and the audit entry id.
// There is no --json rendering for exec (spec exemption): this struct exists
// for the CLI's exit-code mapping and for tests.
type ExecResult struct {
	ExitCode int
	Action   string // audit entry id
}

// Exec runs one command inside the project container per the SPEC-verbs exec
// contract: required command, verbatim exit propagation, workspace cwd,
// login-shell env, start-if-stopped, error-with-hint if absent.
func Exec(opts ExecOpts) (ExecResult, error) {
	return execImpl(opts, defaultRuntime(), time.Now)
}

func execImpl(opts ExecOpts, rt ContainerRuntime, now func() time.Time) (ExecResult, error) {
	// Command required (SPEC-verbs exec): the CLI rejects a bare invocation
	// before reaching here; this is the engine-level backstop.
	if len(opts.Command) == 0 {
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec requires a command: loom exec -- <cmd> [args…]")
	}
	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}
	resolved, err := playbook.Load(path)
	if err != nil {
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec requires a playbook (%s): %w", path, err)
	}
	pb := resolved.Playbook
	cname := containerName(pb.Name)

	exists, err := rt.Exists(cname)
	if err != nil {
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec: cannot reach the container runtime: %w", err)
	}
	if !exists {
		// The verb never creates or provisions (SPEC-verbs exec lifecycle).
		return ExecResult{ExitCode: 1}, fmt.Errorf("no container %s — run `loom build`", cname)
	}
	// Stopped → start, then enter (idempotent bring-up; docker start on a
	// running container is a no-op).
	if err := rt.Start(cname); err != nil {
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec: start %s: %w", cname, err)
	}

	exit, runErr := rt.Exec(cname, opts.Command, containerWorkspace(pb.Name))
	if runErr != nil {
		// Transport-level failure (docker itself, not the command): no exit
		// code to propagate.
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec: %w", runErr)
	}

	// The structured surface (SPEC-verbs exec): one audit entry per exec,
	// carrying the command and its exit code; the entry id is the Append id.
	res := ExecResult{ExitCode: exit}
	if log, lerr := audit.Open(resolved.Root); lerr == nil {
		if id, aerr := log.Append(audit.Entry{
			TS: now().UTC().Format(time.RFC3339), Verb: "exec", Action: "container.exec",
			Target: cname,
			After:  map[string]any{"command": strings.Join(opts.Command, " "), "exit": exit},
			Result: "exited", Actor: "cli",
		}); aerr == nil {
			res.Action = id
		}
	}
	return res, nil
}
