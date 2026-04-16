// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSpec_Valid(t *testing.T) {
	s, err := LoadSpec("testdata/valid/minimal.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Metadata.ID != "acme.demo" {
		t.Fatalf("id=%q", s.Metadata.ID)
	}
	if s.Provider.Ref != "provider.anthropic@1.0.0" {
		t.Fatalf("provider ref=%q", s.Provider.Ref)
	}
}

func TestLoadSpec_RejectsUnknownField(t *testing.T) {
	_, err := LoadSpec("testdata/invalid/unknown_field.yaml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error missing field name: %v", err)
	}
}

func TestValidateFixtures(t *testing.T) {
	matches, err := filepath.Glob("testdata/invalid/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, p := range matches {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			wantBytes, err := os.ReadFile(strings.TrimSuffix(p, ".yaml") + ".err.txt")
			if err != nil {
				t.Fatalf("missing .err.txt: %v", err)
			}
			want := strings.TrimSpace(string(wantBytes))

			s, err := LoadSpec(p)
			if err != nil {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("load error %q does not contain %q", err, want)
				}
				return // loader caught it
			}
			err = s.Validate()
			if err == nil {
				t.Fatalf("fixture %s: expected validation error containing %q", p, want)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("validation error %q does not contain %q", err, want)
			}
		})
	}
}

func TestLoadAndValidate_Full(t *testing.T) {
	s, err := LoadSpec("testdata/valid/full.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
