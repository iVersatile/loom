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

// Shell opens an interactive login shell in the project container —
// SPEC-verbs "shell": sugar over exec's engine path, the same lifecycle and
// no-filtering posture, ONE code path with a TTY option. Exits with the
// shell's exit code. Audit: one session-open entry; commands typed inside
// are never captured.
func Shell(opts ShellOpts) (ExecResult, error) {
	return shellImpl(opts, defaultRuntime(), time.Now)
}

func shellImpl(opts ShellOpts, rt ContainerRuntime, now func() time.Time) (ExecResult, error) {
	// bash -l: the provisioned login env (PATH, prompt dotfiles) applies; the
	// base image guarantees bash (debian bookworm, ADR-0012 territory).
	return execImpl(ExecOpts{
		PlaybookPath: opts.PlaybookPath,
		Command:      []string{"bash", "-l"},
		TTY:          true,
	}, rt, now)
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

	// Fail-closed audit (H2, phase-1 review): the audit entry is exec's whole
	// structured surface (FR-EXEC-003 — "every exec appends an entry"), and
	// exec is the agent-facing door into the container. A door that can't
	// record its own use stays shut: open the log BEFORE touching the
	// container, never silently degrade to an unaudited run.
	log, lerr := audit.Open(resolved.Root)
	if lerr != nil {
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec: action log unavailable, refusing to run unaudited: %w", lerr)
	}

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

	// TTY = shell mode (SPEC-verbs shell): the audit entry is session-open,
	// appended BEFORE entering — a crash mid-session still leaves the open on
	// record — and it captures NO command: what is typed inside belongs to
	// the session, not the log. Fail-closed too: no record, no session.
	var openID string
	if opts.TTY {
		openID, lerr = log.Append(audit.Entry{
			TS: now().UTC().Format(time.RFC3339), Verb: "shell", Action: "container.shell",
			Target: cname, Result: "opened", Actor: "cli",
		})
		if lerr != nil {
			return ExecResult{ExitCode: 1}, fmt.Errorf("shell: session-open audit failed, refusing to open unaudited: %w", lerr)
		}
	}

	// Model A (T10/ADR-0019 amended): entry verbs run AS the configured user
	// (`docker exec -u <user>`); unset/root = the container default (root).
	exit, runErr := rt.Exec(cname, opts.Command, containerWorkspace(pb.Name), pb.User, opts.TTY)
	if runErr != nil {
		// Transport-level failure (docker itself, not the command): no exit
		// code to propagate.
		return ExecResult{ExitCode: 1}, fmt.Errorf("exec: %w", runErr)
	}

	res := ExecResult{ExitCode: exit, Action: openID}
	if !opts.TTY {
		// The structured surface (SPEC-verbs exec): one audit entry per exec,
		// carrying the command and its exit code; the entry id is the Append id.
		// The command already ran, so its exit code survives — but a failed
		// append is a loud error, never a silent gap in the record.
		id, aerr := log.Append(audit.Entry{
			TS: now().UTC().Format(time.RFC3339), Verb: "exec", Action: "container.exec",
			Target: cname,
			After:  map[string]any{"command": strings.Join(opts.Command, " "), "exit": exit},
			Result: "exited", Actor: "cli",
		})
		if aerr != nil {
			return res, fmt.Errorf("exec: command exited %d but the audit append failed: %w", exit, aerr)
		}
		res.Action = id
	}
	return res, nil
}
