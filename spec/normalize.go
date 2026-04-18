// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// MaxExtendsDepth bounds how deep an extends chain may go before
// resolveExtendsChain returns ErrExtendsInvalid (Reason: "depth").
// Picked at design time; tune in a later phase if real specs hit it.
const MaxExtendsDepth = 8

// ExtendsError carries the chain that triggered an extends violation
// so callers can log the exact path. Reason is one of "cycle" or
// "depth".
//
// Wraps ErrExtendsInvalid: callers may match on the sentinel via
// errors.Is, then type-assert to *ExtendsError to inspect Chain and
// Reason.
type ExtendsError struct {
	// Chain lists refs in resolution order, starting from the spec
	// that triggered the failure. For cycles it ends with the ref
	// that closed the loop (also present earlier in the slice).
	Chain []string
	// Reason is "cycle" or "depth".
	Reason string
}

func (e *ExtendsError) Error() string {
	return fmt.Sprintf("extends: %s detected through chain [%s]",
		e.Reason, strings.Join(e.Chain, " -> "))
}

// Unwrap exposes ErrExtendsInvalid so errors.Is(err, ErrExtendsInvalid)
// holds for any *ExtendsError.
func (e *ExtendsError) Unwrap() error { return ErrExtendsInvalid }

// Is matches ErrExtendsInvalid (in addition to the default identity
// match against the same *ExtendsError pointer).
func (e *ExtendsError) Is(target error) bool {
	return target == ErrExtendsInvalid
}

// resolveExtendsChain walks s.Extends and every parent's Extends in
// turn, accumulating loaded parents in root-first order. Each parent's
// own extends is resolved depth-first before the parent itself is
// added to the chain.
//
// Limits:
//   - max depth MaxExtendsDepth (8); deeper → *ExtendsError, Reason "depth"
//   - cycle detected via visited set;       → *ExtendsError, Reason "cycle"
//   - parent missing from store              → wrapped ErrSpecNotFound
//
// Context cancellation aborts the walk and returns ctx.Err().
//
// The returned slice is root-first: parents[0] is the deepest ancestor;
// parents[len-1] is the direct parent of s. The merge step in
// mergeChain consumes them in this order with child-wins semantics.
//
// Returned []string is the resolved chain in root-first order; used as
// NormalizedSpec.ExtendsChain by Normalize.
func resolveExtendsChain(
	ctx context.Context,
	s *AgentSpec,
	store SpecStore,
) (parents []*AgentSpec, chain []string, err error) {
	if len(s.Extends) == 0 {
		return nil, nil, nil
	}
	if store == nil {
		return nil, nil, ErrNoSpecStore
	}

	visited := map[string]bool{}
	var walk func(node *AgentSpec, _ string, depth int, path []string) error
	walk = func(node *AgentSpec, _ string, depth int, path []string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > MaxExtendsDepth {
			return &ExtendsError{Chain: append([]string(nil), path...), Reason: "depth"}
		}
		for _, parentRef := range node.Extends {
			cycledPath := append(path, parentRef) //nolint:gocritic // each iteration builds its own slice for the error path
			if visited[parentRef] {
				return &ExtendsError{Chain: append([]string(nil), cycledPath...), Reason: "cycle"}
			}
			visited[parentRef] = true

			parent, err := store.Load(ctx, parentRef)
			if err != nil {
				if errors.Is(err, ErrSpecNotFound) {
					return fmt.Errorf("resolve %q: %w", parentRef, err)
				}
				return fmt.Errorf("resolve %q: %w", parentRef, err)
			}
			if err := walk(parent, parentRef, depth+1, cycledPath); err != nil {
				return err
			}
			parents = append(parents, parent)
			chain = append(chain, parentRef)
		}
		return nil
	}

	if err := walk(s, "", 0, nil); err != nil {
		return nil, nil, err
	}
	return parents, chain, nil
}
