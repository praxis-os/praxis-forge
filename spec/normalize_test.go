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
