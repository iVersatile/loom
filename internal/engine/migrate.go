package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// credentialSource yields a credential's value by name. The seam exists so a
// 1Password/Keychain source is a drop-in later (FR-RUN-007: pluggable source).
type credentialSource interface {
	value(name string) (string, bool)
}

// envSource reads a credential's value from the process environment.
type envSource struct{}

func (envSource) value(name string) (string, bool) {
	v := os.Getenv(name)
	return v, v != ""
}

// rcFileSource scans common shell rc files for a `NAME=...` / `export NAME=...`
// assignment and returns the LAST match's value, quote-stripped.
type rcFileSource struct{ home string }

func (s rcFileSource) value(name string) (string, bool) {
	var (
		found bool
		val   string
	)
	for _, rc := range []string{".bashrc", ".zshrc", ".profile"} {
		data, err := os.ReadFile(filepath.Join(s.home, rc))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			n, v, ok := parseAssign(line)
			if ok && n == name {
				// LAST match wins, so keep scanning and overwrite.
				found, val = true, v
			}
		}
	}
	return val, found
}

// composedSource tries each source in order; the first that finds the name wins
// (so env beats rc files).
type composedSource []credentialSource

func (cs composedSource) value(name string) (string, bool) {
	for _, s := range cs {
		if v, ok := s.value(name); ok {
			return v, true
		}
	}
	return "", false
}

// defaultCredSource is the production credential source: process env first, then
// the shell rc files under the user's home.
func defaultCredSource() credentialSource {
	home, _ := os.UserHomeDir()
	return composedSource{envSource{}, rcFileSource{home: home}}
}

// isReference reports whether a value is a secret-store reference (op://...).
// References are NOT secrets (FR-RUN-008): they are shown in the clear and
// written verbatim.
func isReference(v string) bool {
	return strings.HasPrefix(v, "op://")
}

// migrateEntry is the value-bearing, ENGINE-INTERNAL plan. It carries the secret
// (or op:// reference) and is NEVER serialized or logged — note the lowercase
// type and absence of json tags.
type migrateEntry struct {
	name      string
	value     string // secret OR op:// ref — NEVER serialized/logged
	reference bool
	status    string // "new" | "changed" | "unchanged"
}

// MigratedCred is the REDACTED, public view of one migrated credential. It is
// safe to serialize, render, and log: Display carries an op:// reference (a
// non-secret) but is "" for a secret.
type MigratedCred struct {
	Name      string `json:"name"`
	Status    string `json:"status"`            // new | changed | unchanged
	Reference bool   `json:"reference"`         // true => op:// (non-secret)
	Display   string `json:"display,omitempty"` // the ref value when Reference; "" for a secret (NEVER the secret)
}

// MigrateReport is the redacted plan the CLI and tests interact with. It never
// carries a secret value.
type MigrateReport struct {
	Creds   []MigratedCred `json:"creds"`
	EnvPath string         `json:"env_path"`
}

// Migrate consolidates the given detected credentials into the .env at envPath.
// It reads values via defaultCredSource(), computes the merge, and — ONLY if a
// write is needed — calls confirm(report) with the REDACTED report (the CLI renders
// the diff + prompts inside it); writes the merged .env only if confirm returns true.
// Secret values never leave this function: confirm sees only MigrateReport. Returns
// the redacted report, the number of entries written (new+changed), and an error.
func Migrate(creds []Credential, envPath string, confirm func(MigrateReport) (bool, error)) (MigrateReport, int, error) {
	return migrateImpl(creds, defaultCredSource(), envPath, confirm)
}

func migrateImpl(creds []Credential, src credentialSource, envPath string, confirm func(MigrateReport) (bool, error)) (MigrateReport, int, error) {
	existing := parseEnvFile(envPath)

	var (
		entries []migrateEntry
		report  MigrateReport
	)
	report.EnvPath = envPath

	for _, c := range creds {
		v, ok := src.value(c.Name)
		if !ok {
			continue
		}
		ref := isReference(v)
		status := "new"
		if cur, present := existing[c.Name]; present {
			if cur == v {
				status = "unchanged"
			} else {
				status = "changed"
			}
		}
		entries = append(entries, migrateEntry{name: c.Name, value: v, reference: ref, status: status})
		mc := MigratedCred{Name: c.Name, Status: status, Reference: ref}
		if ref {
			mc.Display = v
		}
		report.Creds = append(report.Creds, mc)
	}

	// Idempotent no-op: nothing to change ⇒ no confirm, no write.
	writable := writableEntries(entries)
	if len(writable) == 0 {
		return report, 0, nil
	}

	// Gitignore guard: a literal secret write into a .env a git repo does NOT
	// ignore is a real commit risk; refuse. References are not secrets, so they
	// do not trip the guard on their own.
	if hasSecretWrite(writable) && gitTracksUnignored(envPath) {
		dir := filepath.Dir(envPath)
		if dir == "" {
			dir = "."
		}
		return report, 0, fmt.Errorf(".env is not gitignored in %s; refusing to write secrets — add .env to .gitignore first", dir)
	}

	ok, err := confirm(report)
	if err != nil {
		return report, 0, err
	}
	if !ok {
		return report, 0, nil // aborted; nothing written
	}

	if err := writeEnvMerge(envPath, writable); err != nil {
		return report, 0, err
	}
	return report, len(writable), nil
}

// writableEntries returns the entries that would change the .env (new|changed).
func writableEntries(entries []migrateEntry) []migrateEntry {
	var out []migrateEntry
	for _, e := range entries {
		if e.status != "unchanged" {
			out = append(out, e)
		}
	}
	return out
}

// hasSecretWrite reports whether any entry to be written is a literal secret
// (not an op:// reference).
func hasSecretWrite(entries []migrateEntry) bool {
	for _, e := range entries {
		if !e.reference {
			return true
		}
	}
	return false
}

// parseEnvFile reads an .env into a name->value map. It splits on the first `=`,
// strips an optional `export ` prefix and surrounding quotes, and skips blank /
// comment lines. A missing file yields an empty map and no error.
func parseEnvFile(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		if n, v, ok := parseAssign(line); ok {
			out[n] = v
		}
	}
	return out
}

// parseAssign parses one `NAME=value` / `export NAME=value` line, returning the
// name and quote-stripped value. Blank lines and `#` comments yield ok=false.
func parseAssign(line string) (name, value string, ok bool) {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return "", "", false
	}
	t = strings.TrimPrefix(t, "export ")
	t = strings.TrimSpace(t)
	i := strings.Index(t, "=")
	if i <= 0 {
		return "", "", false
	}
	name = strings.TrimSpace(t[:i])
	value = stripQuotes(strings.TrimSpace(t[i+1:]))
	if name == "" {
		return "", "", false
	}
	return name, value, true
}

// stripQuotes removes a single pair of surrounding single or double quotes.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// gitTracksUnignored reports whether path is in a git repo that does NOT ignore it
// (a real commit risk). Best-effort: not-a-repo or git-unavailable => false (no git
// commit risk). Only a definitive "real repo + not ignored" => true.
func gitTracksUnignored(path string) bool {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return false // not a repo (or git unavailable): no commit risk
	}
	// `git check-ignore -q` exits 0 when the path IS ignored, 1 when it is not.
	if err := exec.Command("git", "-C", dir, "check-ignore", "-q", filepath.Base(path)).Run(); err != nil {
		return true // not ignored ⇒ real commit risk
	}
	return false // ignored ⇒ safe
}

// writeEnvMerge writes the entries into the .env at path: an existing `NAME=` (or
// `export NAME=`) line is replaced in place, a new name is appended, and every other
// line (comments, blanks, unrelated vars) is preserved verbatim. Values are written
// verbatim (op:// refs and token secrets are quote-free). File mode 0600.
func writeEnvMerge(path string, entries []migrateEntry) error {
	want := make(map[string]string, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, seen := want[e.name]; !seen {
			order = append(order, e.name)
		}
		want[e.name] = e.value
	}

	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(string(data), "\n")
		// A trailing newline yields a spurious final empty element; drop it so we
		// re-add exactly one trailing newline at the end.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	written := map[string]bool{}
	out := make([]string, 0, len(lines)+len(order))
	for _, line := range lines {
		if n, _, ok := parseAssign(line); ok {
			if v, want := want[n]; want {
				out = append(out, n+"="+v)
				written[n] = true
				continue
			}
		}
		out = append(out, line)
	}
	for _, n := range order {
		if !written[n] {
			out = append(out, n+"="+want[n])
		}
	}

	data := strings.Join(out, "\n") + "\n"
	// Tighten a PRE-EXISTING file to 0600 BEFORE writing secret content into it:
	// os.WriteFile preserves an existing file's mode, so without this a secret
	// would be briefly readable at the old (e.g. 0644) mode. Best-effort — a
	// missing file errors here and is then created at 0600 by WriteFile.
	_ = os.Chmod(path, 0o600)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		return err
	}
	// Belt-and-suspenders: enforce 0600 after the write too.
	return os.Chmod(path, 0o600)
}
