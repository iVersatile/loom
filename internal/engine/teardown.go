package engine

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/iVersatile/loom/internal/audit"
	"github.com/iVersatile/loom/internal/playbook"
)

// Confirmer asks the operator to confirm an irreversible action by typing the
// project name. It returns true only on an exact match. Injected so the gate is
// testable and so --wipe-project can never be satisfied non-interactively.
type Confirmer func(projectName string) bool

// Teardown removes the environment in tiers (docs/SPEC-verbs.md "teardown").
func Teardown(opts TeardownOpts) (TeardownResult, error) {
	return teardownImpl(opts, defaultRuntime(), time.Now, stdinConfirmer)
}

// stdinConfirmer reads a typed project name from stdin. A non-interactive stdin
// (EOF) yields false, so --wipe-project cannot succeed unattended.
func stdinConfirmer(projectName string) bool {
	fmt.Fprintf(os.Stderr, "Type the project name %q to confirm wipe: ", projectName)
	var line string
	if _, err := fmt.Fscanln(os.Stdin, &line); err != nil {
		return false
	}
	return line == projectName
}

func teardownImpl(opts TeardownOpts, rt ContainerRuntime, now func() time.Time, confirm Confirmer) (TeardownResult, error) {
	path := opts.PlaybookPath
	if path == "" {
		path = defaultPlaybookPath
	}
	resolved, err := playbook.Load(path)
	if err != nil {
		return TeardownResult{}, fmt.Errorf("teardown requires a playbook (%s): %w", path, err)
	}
	pb := resolved.Playbook
	root := resolved.Root
	ts := now().UTC().Format(time.RFC3339)

	log, err := audit.Open(root)
	if err != nil {
		return TeardownResult{}, fmt.Errorf("open action log: %w", err)
	}

	res := TeardownResult{Level: opts.Level, Removed: Removed{Containers: []string{}, Volumes: []string{}, Images: []string{}}}

	// Diagnostic log (ADR-0010), like build.
	var logw io.Writer
	if lf, path := openDiagLog(root, "teardown"); lf != nil {
		defer func() { _ = lf.Close() }()
		logw = lf
		res.LogPath = path
		_, _ = fmt.Fprintf(lf, "loom teardown %s level=%s\n", ts, opts.Level)
	}

	cname := containerName(pb.Name)
	removed, err := rt.Teardown(cname, opts.Level, logw)
	if err != nil {
		return res, fmt.Errorf("container teardown: %w", err)
	}
	res.Removed = removed
	for _, c := range removed.Containers {
		_, _ = log.Append(audit.Entry{TS: ts, Verb: "teardown", Action: "container.remove", Target: c, Result: "removed", Actor: "cli"})
	}
	for _, v := range removed.Volumes {
		_, _ = log.Append(audit.Entry{TS: ts, Verb: "teardown", Action: "volume.remove", Target: v, Result: "removed", Actor: "cli"})
	}
	for _, img := range removed.Images {
		_, _ = log.Append(audit.Entry{TS: ts, Verb: "teardown", Action: "image.remove", Target: img, Result: "removed", Actor: "cli"})
	}

	// --wipe-project: whole-folder removal, gated by typed confirmation that
	// cannot be bypassed (docs/SPEC-verbs.md). The guardrail is the gate.
	if opts.WipeProject {
		if !confirm(pb.Name) {
			return res, fmt.Errorf("wipe-project: confirmation did not match %q; nothing wiped", pb.Name)
		}
		if err := os.RemoveAll(root); err != nil {
			return res, fmt.Errorf("wipe-project: %w", err)
		}
		_, _ = log.Append(audit.Entry{TS: ts, Verb: "teardown", Action: "project.wipe", Target: root, Result: "removed", Actor: "cli"})
	}
	return res, nil
}
