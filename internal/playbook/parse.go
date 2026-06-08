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
func Parse(data []byte) (*Playbook, error) {
	var pb Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("parse playbook: %w", err)
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
