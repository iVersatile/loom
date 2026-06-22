package playbook

import (
	"fmt"
	"io/fs"
	"os"

	"sigs.k8s.io/yaml"
)

// Parse decodes a playbook from YAML or JSON bytes. sigs.k8s.io/yaml converts
// YAML to JSON then unmarshals, so the JSON struct tags serve both formats and
// JSON input works natively (frozen format decision, docs/SPEC-playbook.md).
//
// Decoding is STRICT (UnmarshalStrict → json DisallowUnknownFields): a key that
// maps to no declared schema field is a HARD parse error, not silently dropped
// (T20/T28 LOW-2). The whole point is that a typo'd security field — `egres:`
// for `egress:`, `tols:` for `tools:` — can never quietly parse to a permissive
// default; it fails loud and names the offending key. Comments and YAML anchors
// are pre-resolved by the YAML→JSON step and never reach strict decode, so valid
// YAML constructs are unaffected (only keys with no struct field are rejected).
func Parse(data []byte) (*Playbook, error) {
	var pb Playbook
	if err := yaml.UnmarshalStrict(data, &pb); err != nil {
		return nil, fmt.Errorf("parse playbook: %w (a misspelled or unrecognized key is rejected — see docs/SPEC-playbook.md for the declared schema)", err)
	}
	return &pb, nil
}

// ParseFile reads and parses a playbook from the host filesystem.
func ParseFile(path string) (*Playbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pb, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return pb, nil
}

// ParseFS reads and parses a playbook from a config-source filesystem.
func ParseFS(fsys fs.FS, name string) (*Playbook, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	pb, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return pb, nil
}
