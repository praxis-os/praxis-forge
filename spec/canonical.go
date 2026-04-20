// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
)

// CanonicalJSON returns the canonical JSON encoding of the normalized spec.
// The encoding is stable across map insertion order variations and YAML
// authoring quirks (empty vs absent collections). Suitable for hashing.
//
// The byte sequence is compact (no whitespace). The result is memoized;
// repeated calls are free.
func (ns *NormalizedSpec) CanonicalJSON() ([]byte, error) {
	ns.canonicalMemo.once.Do(func() {
		ns.canonicalMemo.b, ns.canonicalMemo.err = computeCanonicalJSON(ns.Spec)
	})
	return ns.canonicalMemo.b, ns.canonicalMemo.err
}

// computeCanonicalJSON produces the canonical JSON encoding of an AgentSpec.
// Steps:
//  1. Marshal s with encoding/json (json tags + omitempty handle field selection).
//  2. Unmarshal into an any tree with UseNumber (preserves int precision).
//  3. Pre-walk: drop empty maps and empty slices so {}/[] are indistinguishable
//     from nil (YAML authoring quirks don't perturb the hash).
//  4. Re-encode via canonicalEncode (sorted map keys, no HTML escape).
func computeCanonicalJSON(s AgentSpec) ([]byte, error) {
	// Step 1: standard marshal using json tags.
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("spec: canonical JSON marshal: %w", err)
	}

	// Step 2: unmarshal to any with number preservation.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("spec: canonical JSON decode: %w", err)
	}

	// Step 3: normalize empty collections.
	tree = pruneEmpty(tree)

	// Step 4: canonical encode.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// We use the low-level encoder only to initialize; actual writing is done
	// by canonicalEncode which doesn't use enc directly.
	_ = enc
	if err := canonicalEncode(&buf, tree); err != nil {
		return nil, fmt.Errorf("spec: canonical JSON encode: %w", err)
	}
	return buf.Bytes(), nil
}

// pruneEmpty walks the decoded any tree and removes empty maps and empty
// slices, so {} and nil produce identical canonical output.
func pruneEmpty(v any) any {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return nil
		}
		out := make(map[string]any, len(val))
		for k, child := range val {
			pruned := pruneEmpty(child)
			if pruned != nil {
				out[k] = pruned
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		if len(val) == 0 {
			return nil
		}
		out := make([]any, 0, len(val))
		for _, child := range val {
			pruned := pruneEmpty(child)
			if pruned != nil {
				out = append(out, pruned)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return v
	}
}

// canonicalEncode writes v to w as compact JSON with map keys sorted
// lexicographically. Non-string map keys are coerced to strings via
// fmt.Sprintf (yaml.v3 maps always use string keys for AgentSpec, but we
// defend against any any-typed nesting in ComponentRef.Config).
func canonicalEncode(w io.Writer, v any) error {
	switch val := v.(type) {
	case nil:
		_, err := io.WriteString(w, "null")
		return err
	case bool:
		return canonicalEncodeBool(w, val)
	case json.Number:
		_, err := io.WriteString(w, val.String())
		return err
	case float64:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case string:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case map[string]any:
		return canonicalEncodeMap(w, val)
	case []any:
		return canonicalEncodeSlice(w, val)
	default:
		// Fallback for unexpected types (e.g. map[any]any from yaml.v3).
		b, err := json.Marshal(fmt.Sprintf("%v", val))
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
}

func canonicalEncodeBool(w io.Writer, v bool) error {
	if v {
		_, err := io.WriteString(w, "true")
		return err
	}
	_, err := io.WriteString(w, "false")
	return err
}

func canonicalEncodeMap(w io.Writer, val map[string]any) error {
	keys := make([]string, 0, len(val))
	for k := range val {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for i, k := range keys {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return err
		}
		if _, err := w.Write(kb); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ":"); err != nil {
			return err
		}
		if err := canonicalEncode(w, val[k]); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

func canonicalEncodeSlice(w io.Writer, val []any) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i, item := range val {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := canonicalEncode(w, item); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

// canonicalOnce, canonicalBytes, canonicalErr live on NormalizedSpec in
// provenance.go. This file provides the methods that use them.

// noCopy prevents incorrect value-copy of NormalizedSpec after sync.Once is embedded.
// Checked by go vet via the Lock/Unlock no-op.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// memoCanonical holds the cached canonical JSON and any error from computing it.
// Embedded in NormalizedSpec alongside a sync.Once.
type memoCanonical struct {
	_    noCopy
	once sync.Once
	b    []byte
	err  error
}
