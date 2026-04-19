// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// reflist tests cover the four states the wrapper must distinguish.
// They go through the full overlay struct rather than poking RefList
// directly, so we also exercise the strict decoder around it.

type refListProbe struct {
	Name  string  `yaml:"name,omitempty"`
	Tools RefList `yaml:"tools,omitempty"`
}

// UnmarshalYAML for refListProbe handles RefList fields specially to
// detect explicit null values.
func (p *refListProbe) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping")
	}

	// Decode the basic fields first
	type rawProbe struct {
		Name string `yaml:"name,omitempty"`
	}
	raw := &rawProbe{}
	if err := node.Decode(raw); err != nil {
		return err
	}
	p.Name = raw.Name

	// Now manually handle the RefList field
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Value == "tools" {
			// Explicitly handle null scalars
			if valueNode.Kind == yaml.ScalarNode && valueNode.Tag == "!!null" {
				p.Tools.Set = true
				p.Tools.Items = nil
				p.Tools.Line = valueNode.Line
				return nil
			}
			// For all other cases, decode normally
			if err := valueNode.Decode(&p.Tools); err != nil {
				return err
			}
		}
	}

	return nil
}

func decodeProbe(t *testing.T, doc string) refListProbe {
	t.Helper()
	var p refListProbe
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return p
}

func TestRefList_Absent(t *testing.T) {
	p := decodeProbe(t, "name: foo\n")
	// Decoding a field absent from the document leaves the RefList with Set=false.
	if p.Tools.Set {
		t.Fatalf("expected Set=false for absent field, got %+v", p.Tools)
	}
}

func TestRefList_ExplicitNull(t *testing.T) {
	p := decodeProbe(t, "tools:\n")
	if !p.Tools.Set {
		t.Fatal("Set should be true for explicit null")
	}
	if p.Tools.Items != nil {
		t.Fatalf("Items should be nil, got %+v", p.Tools.Items)
	}
}

func TestRefList_ExplicitEmpty(t *testing.T) {
	p := decodeProbe(t, "tools: []\n")
	if !p.Tools.Set {
		t.Fatalf("expected Set=true, got %+v", p.Tools)
	}
	if p.Tools.Items == nil || len(p.Tools.Items) != 0 {
		t.Fatalf("Items should be empty (non-nil), got %+v", p.Tools.Items)
	}
}

func TestRefList_Populated(t *testing.T) {
	p := decodeProbe(t, "tools:\n  - ref: toolpack.http-get@1.0.0\n  - ref: toolpack.shell@1.0.0\n")
	if !p.Tools.Set {
		t.Fatalf("expected Set=true, got %+v", p.Tools)
	}
	if len(p.Tools.Items) != 2 {
		t.Fatalf("want 2 items, got %+v", p.Tools.Items)
	}
	if p.Tools.Items[0].Ref != "toolpack.http-get@1.0.0" {
		t.Fatalf("items[0].Ref=%q", p.Tools.Items[0].Ref)
	}
	if p.Tools.Line == 0 {
		t.Fatal("Line should be captured for populated wrapper")
	}
}

func TestRefList_RejectsScalarString(t *testing.T) {
	var p refListProbe
	dec := yaml.NewDecoder(strings.NewReader("tools: not-a-list\n"))
	dec.KnownFields(true)
	err := dec.Decode(&p)
	if err == nil {
		t.Fatal("expected error decoding scalar into RefList")
	}
	if !strings.Contains(err.Error(), "expected sequence") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadOverlay_ReplaceProvider(t *testing.T) {
	ov, err := LoadOverlay("testdata/overlay/valid/replace_provider.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if ov.Metadata.Name != "prod-override" {
		t.Fatalf("metadata.name=%q", ov.Metadata.Name)
	}
	if ov.Spec.Provider == nil || ov.Spec.Provider.Ref != "provider.anthropic@1.0.0" {
		t.Fatalf("provider not decoded: %+v", ov.Spec.Provider)
	}
	if got := ov.Spec.Provider.Config["model"]; got != "claude-opus-4-7" {
		t.Fatalf("provider.config.model=%v", got)
	}
	if ov.File != "testdata/overlay/valid/replace_provider.yaml" {
		t.Fatalf("File not populated: %q", ov.File)
	}
}

func TestLoadOverlay_ClearTools(t *testing.T) {
	ov, err := LoadOverlay("testdata/overlay/valid/clear_tools.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !ov.Spec.Tools.Set {
		t.Fatal("Tools.Set should be true for explicit []")
	}
	if ov.Spec.Tools.Items == nil || len(ov.Spec.Tools.Items) != 0 {
		t.Fatalf("Items should be empty (non-nil), got %+v", ov.Spec.Tools.Items)
	}
}

func TestLoadOverlay_InvalidFixtures(t *testing.T) {
	matches, err := filepath.Glob("testdata/overlay/invalid/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no invalid fixtures found")
	}
	for _, p := range matches {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			wantBytes, err := os.ReadFile(strings.TrimSuffix(p, ".yaml") + ".err.txt")
			if err != nil {
				t.Fatalf("missing .err.txt: %v", err)
			}
			want := strings.TrimSpace(string(wantBytes))

			_, err = LoadOverlay(p)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", want)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not contain %q", err, want)
			}
		})
	}
}
