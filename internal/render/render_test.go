package render

import (
	"bytes"
	"encoding/json"
	"testing"
)

type fake struct {
	Name string `json:"name"`
}

func (f fake) Human() string { return "name=" + f.Name }

func TestEmitJSON(t *testing.T) {
	var b bytes.Buffer
	if err := Emit(&b, true, fake{Name: "x"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	var got fake
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, b.String())
	}
	if got.Name != "x" {
		t.Errorf("round-trip name = %q, want x", got.Name)
	}
}

func TestEmitHuman(t *testing.T) {
	var b bytes.Buffer
	if err := Emit(&b, false, fake{Name: "x"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if got := b.String(); got != "name=x\n" {
		t.Errorf("human output = %q, want %q", got, "name=x\n")
	}
}
