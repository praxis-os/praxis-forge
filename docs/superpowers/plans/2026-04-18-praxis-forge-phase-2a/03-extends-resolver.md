# Task group 3 — extends chain resolver

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(spec): extends chain resolver`

**Scope:** the `resolveExtendsChain` function plus its typed `ExtendsError` carrier. Resolves `s.Extends` (and every parent's `Extends`) through a `SpecStore`, with depth bound 8 and cycle detection via a visited set. Returns parents in root-first order (the order `mergeChain` will consume them in task group 4). Honors `ctx` cancellation. **No** merge or overlay logic yet — that lands in task group 4.

---

## Task 3.1: `ExtendsError` typed carrier

**Files:**
- Create: `spec/normalize.go` (initial chunk — types + ExtendsError only)

- [ ] **Step 1: Write the file header + `ExtendsError`**

```go
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
```

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 3.2: `resolveExtendsChain` happy path

**Files:**
- Modify: `spec/normalize.go`

- [ ] **Step 1: Append the resolver**

Append to `spec/normalize.go`:

```go
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
	var walk func(node *AgentSpec, ref string, depth int, path []string) error
	walk = func(node *AgentSpec, ref string, depth int, path []string) error {
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
```

> **Note for engineer:** `parents` and `chain` accumulate in append order, which is depth-first post-order — the deepest parent is appended first, then we unwind back to s's direct parents. That ordering matches the spec's "parents earlier in s.Extends are merged in earlier" rule because each direct parent of s contributes its own subtree before the next direct parent's subtree begins.

- [ ] **Step 2: Build**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 3.3: Resolver tests — happy path + depth + cycle + missing + ctx

**Files:**
- Create: `spec/normalize_test.go`

- [ ] **Step 1: Write the test file**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubSpec returns a minimal valid spec that uses extends to point at
// the given parent refs. Returned spec is sufficient for resolver
// tests; full validation is exercised in normalize_test.go's
// integration cases.
func stubSpec(extends ...string) *AgentSpec {
	return &AgentSpec{
		APIVersion: expectedAPIVersion,
		Kind:       expectedKind,
		Metadata:   Metadata{ID: "acme.demo", Version: "0.1.0"},
		Provider:   ComponentRef{Ref: "provider.fake@1.0.0"},
		Prompt:     PromptBlock{System: &ComponentRef{Ref: "prompt.sys@1.0.0"}},
		Extends:    extends,
	}
}

func TestResolveExtendsChain_NoExtends(t *testing.T) {
	parents, chain, err := resolveExtendsChain(context.Background(), stubSpec(), nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if parents != nil || chain != nil {
		t.Fatalf("expected nil, got parents=%v chain=%v", parents, chain)
	}
}

func TestResolveExtendsChain_NoStoreErrors(t *testing.T) {
	_, _, err := resolveExtendsChain(context.Background(), stubSpec("a@1.0.0"), nil)
	if !errors.Is(err, ErrNoSpecStore) {
		t.Fatalf("err=%v, want ErrNoSpecStore", err)
	}
}

func TestResolveExtendsChain_LinearChainRootFirst(t *testing.T) {
	store := MapSpecStore{
		"acme.parent@1.0.0":      stubSpec("acme.grandparent@1.0.0"),
		"acme.grandparent@1.0.0": stubSpec(),
	}
	parents, chain, err := resolveExtendsChain(context.Background(), stubSpec("acme.parent@1.0.0"), store)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(parents) != 2 {
		t.Fatalf("want 2 parents, got %d", len(parents))
	}
	wantChain := []string{"acme.grandparent@1.0.0", "acme.parent@1.0.0"}
	for i, want := range wantChain {
		if chain[i] != want {
			t.Errorf("chain[%d]=%q, want %q", i, chain[i], want)
		}
	}
}

func TestResolveExtendsChain_BranchedChain(t *testing.T) {
	// s extends [A, B]; A extends [A1]; B extends [B1].
	// Expected order: A1, A, B1, B (depth-first per direct parent).
	store := MapSpecStore{
		"a@1.0.0":  stubSpec("a1@1.0.0"),
		"a1@1.0.0": stubSpec(),
		"b@1.0.0":  stubSpec("b1@1.0.0"),
		"b1@1.0.0": stubSpec(),
	}
	_, chain, err := resolveExtendsChain(context.Background(), stubSpec("a@1.0.0", "b@1.0.0"), store)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	want := []string{"a1@1.0.0", "a@1.0.0", "b1@1.0.0", "b@1.0.0"}
	if len(chain) != len(want) {
		t.Fatalf("want %d parents, got %d (%v)", len(want), len(chain), chain)
	}
	for i, w := range want {
		if chain[i] != w {
			t.Errorf("chain[%d]=%q, want %q (full=%v)", i, chain[i], w, chain)
		}
	}
}

func TestResolveExtendsChain_DepthExceeded(t *testing.T) {
	// Build a chain MaxExtendsDepth+1 deep.
	store := MapSpecStore{}
	for i := 1; i <= MaxExtendsDepth+1; i++ {
		var nextExt []string
		if i < MaxExtendsDepth+1 {
			nextExt = []string{refForLevel(i + 1)}
		}
		store[refForLevel(i)] = stubSpec(nextExt...)
	}
	_, _, err := resolveExtendsChain(context.Background(), stubSpec(refForLevel(1)), store)
	if !errors.Is(err, ErrExtendsInvalid) {
		t.Fatalf("err=%v, want ErrExtendsInvalid (depth)", err)
	}
	var ee *ExtendsError
	if !errors.As(err, &ee) {
		t.Fatalf("err did not unwrap to *ExtendsError: %v", err)
	}
	if ee.Reason != "depth" {
		t.Fatalf("Reason=%q, want depth", ee.Reason)
	}
	if len(ee.Chain) == 0 {
		t.Fatal("Chain should be populated for depth violation")
	}
}

func TestResolveExtendsChain_DepthAtBoundaryPasses(t *testing.T) {
	store := MapSpecStore{}
	for i := 1; i <= MaxExtendsDepth; i++ {
		var nextExt []string
		if i < MaxExtendsDepth {
			nextExt = []string{refForLevel(i + 1)}
		}
		store[refForLevel(i)] = stubSpec(nextExt...)
	}
	_, chain, err := resolveExtendsChain(context.Background(), stubSpec(refForLevel(1)), store)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(chain) != MaxExtendsDepth {
		t.Fatalf("expected %d parents, got %d", MaxExtendsDepth, len(chain))
	}
}

func TestResolveExtendsChain_DirectCycle(t *testing.T) {
	store := MapSpecStore{
		"a@1.0.0": stubSpec("a@1.0.0"),
	}
	_, _, err := resolveExtendsChain(context.Background(), stubSpec("a@1.0.0"), store)
	if !errors.Is(err, ErrExtendsInvalid) {
		t.Fatalf("err=%v, want ErrExtendsInvalid (cycle)", err)
	}
	var ee *ExtendsError
	if !errors.As(err, &ee) || ee.Reason != "cycle" {
		t.Fatalf("expected cycle ExtendsError, got %v", err)
	}
	if !strings.Contains(ee.Error(), "a@1.0.0") {
		t.Fatalf("error should mention the cycle ref: %v", ee)
	}
}

func TestResolveExtendsChain_TransitiveCycle(t *testing.T) {
	store := MapSpecStore{
		"a@1.0.0": stubSpec("b@1.0.0"),
		"b@1.0.0": stubSpec("c@1.0.0"),
		"c@1.0.0": stubSpec("a@1.0.0"),
	}
	_, _, err := resolveExtendsChain(context.Background(), stubSpec("a@1.0.0"), store)
	if !errors.Is(err, ErrExtendsInvalid) {
		t.Fatalf("err=%v, want ErrExtendsInvalid", err)
	}
	var ee *ExtendsError
	if !errors.As(err, &ee) || ee.Reason != "cycle" {
		t.Fatalf("expected cycle ExtendsError, got %v", err)
	}
}

func TestResolveExtendsChain_MissingParent(t *testing.T) {
	store := MapSpecStore{} // empty
	_, _, err := resolveExtendsChain(context.Background(), stubSpec("missing@1.0.0"), store)
	if !errors.Is(err, ErrSpecNotFound) {
		t.Fatalf("err=%v, want wrap of ErrSpecNotFound", err)
	}
}

func TestResolveExtendsChain_CtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := MapSpecStore{"a@1.0.0": stubSpec()}
	_, _, err := resolveExtendsChain(ctx, stubSpec("a@1.0.0"), store)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}

// refForLevel mints a stable ref id from a depth integer for the
// chain-depth tests.
func refForLevel(i int) string {
	return "level." + itoa(i) + "@1.0.0"
}

// Local itoa avoids importing strconv just for the test helper.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var (
		neg bool
		buf [20]byte
		pos = len(buf)
	)
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
```

- [ ] **Step 2: Run (expect pass for all subtests)**

Run: `go test ./spec/... -run TestResolveExtendsChain -v`
Expected: every subtest PASS.

---

## Task 3.4: Lint + commit task group 3

- [ ] **Step 1: Race + full spec suite**

Run: `go test -race ./spec/... -count=1`
Expected: PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Commit**

```bash
git add spec/normalize.go spec/normalize_test.go
git commit -m "feat(spec): extends chain resolver

Adds resolveExtendsChain plus the typed ExtendsError carrier. Walks
s.Extends depth-first, accumulating loaded parents in root-first order
ready for the merge step. Bounded by MaxExtendsDepth = 8; cycles
caught via a visited set. Parent loads go through SpecStore; missing
parents bubble ErrSpecNotFound. ctx cancellation aborts the walk.

ExtendsError wraps ErrExtendsInvalid (so errors.Is matches the
sentinel) and carries the resolution chain plus Reason ('cycle' or
'depth') for callers that want to log the exact failure path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
