// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SpecStore loads parent specs referenced by AgentSpec.Extends. Forge
// invokes Load serially during chain resolution; implementations need
// not be safe for concurrent use.
//
// Implementations MUST return an error wrapping ErrSpecNotFound when ref
// is not resolvable, so callers can discriminate via errors.Is.
type SpecStore interface {
	Load(ctx context.Context, ref string) (*AgentSpec, error)
}

// MapSpecStore is an in-memory store suitable for tests and for callers
// that load specs from non-filesystem sources (HTTP, embedded FS, etc.)
// and pre-decode them. The map is consulted by exact key.
type MapSpecStore map[string]*AgentSpec

// Load returns the spec for ref, or an error wrapping ErrSpecNotFound
// when no entry exists.
func (m MapSpecStore) Load(ctx context.Context, ref string) (*AgentSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s, ok := m[ref]
	if !ok {
		return nil, fmt.Errorf("MapSpecStore: %s: %w", ref, ErrSpecNotFound)
	}
	return s, nil
}

// FilesystemSpecStore resolves refs as filesystem paths relative to
// Root. Refs that escape Root via ".." or absolute paths return
// ErrSpecNotFound.
//
// FilesystemSpecStore.Load reads the file and runs the same strict YAML
// decoder used by LoadSpec, but it does NOT run Validate — parent
// fragments are validated only as part of the merged result inside
// Normalize.
type FilesystemSpecStore struct {
	Root string
}

// Load reads ref relative to Root and decodes it as an AgentSpec.
func (s *FilesystemSpecStore) Load(ctx context.Context, ref string) (*AgentSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	clean, ok := s.resolve(ref)
	if !ok {
		return nil, fmt.Errorf("FilesystemSpecStore: %s: %w", ref, ErrSpecNotFound)
	}
	f, err := os.Open(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("FilesystemSpecStore: %s: %w", ref, ErrSpecNotFound)
		}
		return nil, fmt.Errorf("FilesystemSpecStore: open %s: %w", ref, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var spec AgentSpec
	if err := dec.Decode(&spec); err != nil {
		return nil, fmt.Errorf("FilesystemSpecStore: decode %s: %w", ref, err)
	}
	return &spec, nil
}

// resolve cleans ref relative to Root and reports whether it stays
// inside Root.
func (s *FilesystemSpecStore) resolve(ref string) (string, bool) {
	if filepath.IsAbs(ref) {
		return "", false
	}
	rootAbs, err := filepath.Abs(s.Root)
	if err != nil {
		return "", false
	}
	candidate := filepath.Join(rootAbs, filepath.Clean(ref))
	rel, err := filepath.Rel(rootAbs, candidate)
	if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return "", false
	}
	return candidate, true
}
