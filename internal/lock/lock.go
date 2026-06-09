// Package lock reads and writes loom.lock — the thick, exact realization of the
// thin playbook (ADR-0002). It records both per-tool pins (intent + resolved +
// source + digest) and the base-image digest (frozen lockfile decision Q3,
// docs/SPEC-playbook.md). Generated, never hand-edited; committed for
// reproducibility.
package lock

import (
	"fmt"
	"os"
	"reflect"

	"sigs.k8s.io/yaml"
)

// Version is the only loom.lock schema version this engine accepts.
const Version = 1

// LockedTool is one tool's exact realization.
type LockedTool struct {
	Intent   string `json:"intent"`
	Resolved string `json:"resolved"`
	Source   string `json:"source"`
	Digest   string `json:"digest,omitempty"`
}

// LockedAgent is one agent harness's resolved version.
type LockedAgent struct {
	Resolved string `json:"resolved"`
}

// Lock is the full lockfile document.
type Lock struct {
	LoomLock   int                    `json:"loom_lock"`
	ResolvedAt string                 `json:"resolved_at"`
	BaseImage  string                 `json:"base_image"`
	Tools      map[string]LockedTool  `json:"tools"`
	Agents     map[string]LockedAgent `json:"agents"`
}

// Read loads a lockfile. A missing file returns (nil, nil) — "no lock yet" is a
// normal state (first build), not an error.
func Read(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var l Lock
	if err := yaml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &l, nil
}

// WriteFile writes the lockfile atomically (temp + rename) so a crash never
// leaves a half-written lock (SPEC-verbs cross-cutting: recoverable).
func (l *Lock) WriteFile(path string) error {
	data, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ContentEqual compares two locks ignoring ResolvedAt, so re-resolving with an
// unchanged result is idempotent: build skips the rewrite and the timestamp does
// not churn (SPEC-verbs "build" idempotency).
func ContentEqual(a, b *Lock) bool {
	if a == nil || b == nil {
		return a == b
	}
	ca, cb := *a, *b
	ca.ResolvedAt, cb.ResolvedAt = "", ""
	return reflect.DeepEqual(ca, cb)
}
