// Package render is the single place that turns a verb result into either
// human-readable text or --json output. Centralizing it keeps the AI-first
// dual-output invariant (ADR-0005, RULES §5) off every individual verb: a verb
// returns a typed result, render decides the surface.
package render

import (
	"encoding/json"
	"fmt"
	"io"
)

// Renderable is implemented by every verb result. Human() is the operator-facing
// rendering; the JSON surface comes from the type's struct tags.
type Renderable interface {
	Human() string
}

// Emit writes v to w as indented JSON when jsonMode is set, otherwise as its
// human rendering. Per the global convention, callers pass stdout here and keep
// logs on stderr (docs/SPEC-verbs.md "Global conventions").
func Emit(w io.Writer, jsonMode bool, v Renderable) error {
	if jsonMode {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	_, err := fmt.Fprintln(w, v.Human())
	return err
}
