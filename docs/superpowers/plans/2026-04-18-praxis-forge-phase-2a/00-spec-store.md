# Task group 0 — `SpecStore` interface + filesystem and map impls

> Part of [praxis-forge Phase 2a Implementation Plan](README.md) — see companion design spec [`../../specs/2026-04-18-praxis-forge-phase-2a-design.md`](../../specs/2026-04-18-praxis-forge-phase-2a-design.md).

**Commit (atomic):** `feat(spec): SpecStore interface + filesystem and map impls`

**Scope:** ship the injectable parent loader for the future `extends:` resolver. Two implementations: filesystem-backed (root-rooted, `..`-rejecting) and in-memory (test-friendly). Add the `ErrSpecNotFound` sentinel that both honor. **Also** lands the broader sentinel set the rest of Phase 2a needs (`ErrNoSpecStore`, `ErrExtendsInvalid`, `ErrLockedFieldOverride`, `ErrPhaseGatedInOverlay`, `ErrCompositionLimit`) plus the `Errors.Wrap` helper, so later task groups can wrap them without revisiting `errors.go`.

---

## Task 0.1: New sentinels + `Errors.Wrap` helper

**Files:**
- Modify: `spec/errors.go`
- Modify: `spec/errors_test.go`

- [ ] **Step 1: Append failing tests**

Append to `spec/errors_test.go`:

```go
func TestErrors_WrapMatchesSentinel(t *testing.T) {
	var e Errors
	e.Wrap(ErrLockedFieldOverride, "metadata.id: changed by overlay %q", "prod")
	if !errors.Is(e, ErrValidation) {
		t.Fatal("aggregator should still match ErrValidation")
	}
	if !errors.Is(e, ErrLockedFieldOverride) {
		t.Fatal("aggregator should match the wrapped sentinel")
	}
	if !errors.Is(e, ErrNoSpecStore) {
		// negative control — must not match unrelated sentinel
	} else {
		t.Fatal("aggregator must not match unrelated sentinel")
	}
	if !strings.Contains(e.Error(), "prod") {
		t.Fatalf("formatted message lost: %v", e)
	}
}

func TestErrors_WrapMultipleSentinels(t *testing.T) {
	var e Errors
	e.Wrap(ErrLockedFieldOverride, "metadata.id changed")
	e.Wrap(ErrCompositionLimit, "overlay count > 16")
	if !errors.Is(e, ErrLockedFieldOverride) {
		t.Fatal("first sentinel lost")
	}
	if !errors.Is(e, ErrCompositionLimit) {
		t.Fatal("second sentinel lost")
	}
}

func TestErrors_AddfDoesNotRecordSentinel(t *testing.T) {
	var e Errors
	e.Addf("plain message")
	if errors.Is(e, ErrLockedFieldOverride) {
		t.Fatal("plain Addf must not match an unrelated sentinel")
	}
}
```

- [ ] **Step 2: Run (expect fail — undefined sentinels and method)**

Run: `go test ./spec/... -run TestErrors -v`
Expected: build error mentioning `ErrLockedFieldOverride`, `ErrNoSpecStore`, `ErrCompositionLimit`, and `Errors.Wrap`.

- [ ] **Step 3: Replace `spec/errors.go` with the extended aggregator**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation is the sentinel for any spec validation failure.
var ErrValidation = errors.New("spec validation failed")

// Phase 2a sentinels. All are matched through errors.Is on the
// aggregated Errors value; callers do not need to unwrap individual
// violations to discriminate.
var (
	ErrNoSpecStore         = errors.New("forge: extends present but no SpecStore configured")
	ErrSpecNotFound        = errors.New("spec store: ref not found")
	ErrExtendsInvalid      = errors.New("extends: invalid")
	ErrLockedFieldOverride = errors.New("locked field overridden")
	ErrPhaseGatedInOverlay = errors.New("phase-gated field in overlay")
	ErrCompositionLimit    = errors.New("composition limit exceeded")
)

// Errors aggregates one or more validation violations reported by Validate
// or Normalize. It records both formatted messages (in declaration order)
// and any sentinels supplied via Wrap, so callers can both render the full
// human-facing string and discriminate via errors.Is.
type Errors struct {
	msgs      []string
	sentinels []error
}

// Addf records a plain formatted message. Use Wrap when the message is
// associated with a sentinel that callers might match on.
func (e *Errors) Addf(format string, args ...any) {
	e.msgs = append(e.msgs, fmt.Sprintf(format, args...))
}

// Wrap records a formatted message *and* tracks the sentinel so the
// aggregated Errors value reports true for errors.Is(err, sentinel).
func (e *Errors) Wrap(sentinel error, format string, args ...any) {
	e.msgs = append(e.msgs, fmt.Sprintf(format, args...))
	e.sentinels = append(e.sentinels, sentinel)
}

// Len reports how many violations have been recorded.
func (e Errors) Len() int { return len(e.msgs) }

// OrNil returns nil if no violation was recorded; otherwise it returns
// e itself (which satisfies the error interface).
func (e Errors) OrNil() error {
	if len(e.msgs) == 0 {
		return nil
	}
	return e
}

func (e Errors) Error() string {
	return fmt.Sprintf("%s: %s", ErrValidation.Error(), strings.Join(e.msgs, "; "))
}

// Is matches ErrValidation always (the default sentinel), plus any
// sentinel wrapped via Wrap.
func (e Errors) Is(target error) bool {
	if target == ErrValidation {
		return true
	}
	for _, s := range e.sentinels {
		if errors.Is(s, target) {
			return true
		}
	}
	return false
}
```

> **Note for engineer:** the Phase 1 `Errors` type was a `[]string` alias. The new shape is a struct so the sentinel slice can ride alongside without breaking the `Addf`/`OrNil`/`Error`/`Is` API. Existing call sites in `spec/validate.go` (`var errs Errors; errs.Addf(...); return errs.OrNil()`) keep compiling unchanged.

- [ ] **Step 4: Run validator tests to confirm Phase 1 path still passes**

Run: `go test ./spec/... -v`
Expected: every previous test (`TestErrors_*`, `TestValidate_*`, `TestLoadSpec_*`, `TestParseID_*`, fixture tests) still passes plus the three new sentinel tests.

- [ ] **Step 5: Run vet + lint**

Run: `go vet ./spec/...`
Run: `make lint`
Expected: clean.

- [ ] **Step 6: Commit later (combined with the rest of task group 0).**

Do **not** commit yet — `errors.go`, `store.go`, and `store_test.go` ship together as commit 1.

---

## Task 0.2: `SpecStore` interface

**Files:**
- Create: `spec/store.go`

- [ ] **Step 1: Implement the interface and the `MapSpecStore`**

```go
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
```

- [ ] **Step 2: Build to surface compile errors early**

Run: `go build ./spec/...`
Expected: clean build.

---

## Task 0.3: `MapSpecStore` tests

**Files:**
- Create: `spec/store_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMapSpecStore_Hit(t *testing.T) {
	m := MapSpecStore{"acme.base@1.0.0": &AgentSpec{Kind: "AgentSpec"}}
	got, err := m.Load(context.Background(), "acme.base@1.0.0")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || got.Kind != "AgentSpec" {
		t.Fatalf("wrong spec: %+v", got)
	}
}

func TestMapSpecStore_Miss(t *testing.T) {
	m := MapSpecStore{}
	_, err := m.Load(context.Background(), "nope@1.0.0")
	if !errors.Is(err, ErrSpecNotFound) {
		t.Fatalf("err=%v, want wrap of ErrSpecNotFound", err)
	}
}

func TestMapSpecStore_CtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := MapSpecStore{}.Load(ctx, "x")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}
```

- [ ] **Step 2: Run (expect pass)**

Run: `go test ./spec/... -run TestMapSpecStore -v`
Expected: PASS.

---

## Task 0.4: `FilesystemSpecStore` tests + fixture

**Files:**
- Create: `spec/testdata/store/parent.yaml`
- Modify: `spec/store_test.go`

- [ ] **Step 1: Write fixture**

```yaml
# spec/testdata/store/parent.yaml
apiVersion: forge.praxis-os.dev/v0
kind: AgentSpec
metadata:
  id: acme.base
  version: 1.0.0
provider:
  ref: provider.anthropic@1.0.0
prompt:
  system:
    ref: prompt.demo-system@1.0.0
```

- [ ] **Step 2: Append failing tests**

Append to `spec/store_test.go`:

```go
func TestFilesystemSpecStore_Hit(t *testing.T) {
	root, err := filepath.Abs("testdata/store")
	if err != nil {
		t.Fatal(err)
	}
	s := &FilesystemSpecStore{Root: root}
	got, err := s.Load(context.Background(), "parent.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Metadata.ID != "acme.base" {
		t.Fatalf("metadata.id=%q", got.Metadata.ID)
	}
}

func TestFilesystemSpecStore_Miss(t *testing.T) {
	root, err := filepath.Abs("testdata/store")
	if err != nil {
		t.Fatal(err)
	}
	s := &FilesystemSpecStore{Root: root}
	_, err = s.Load(context.Background(), "missing.yaml")
	if !errors.Is(err, ErrSpecNotFound) {
		t.Fatalf("err=%v, want wrap of ErrSpecNotFound", err)
	}
}

func TestFilesystemSpecStore_RejectsDotDotEscape(t *testing.T) {
	root, err := filepath.Abs("testdata/store")
	if err != nil {
		t.Fatal(err)
	}
	s := &FilesystemSpecStore{Root: root}
	_, err = s.Load(context.Background(), "../valid/minimal.yaml")
	if !errors.Is(err, ErrSpecNotFound) {
		t.Fatalf("err=%v, want wrap of ErrSpecNotFound for escape", err)
	}
}

func TestFilesystemSpecStore_RejectsAbsolutePath(t *testing.T) {
	root, err := filepath.Abs("testdata/store")
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs("testdata/store/parent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := &FilesystemSpecStore{Root: root}
	_, err = s.Load(context.Background(), abs)
	if !errors.Is(err, ErrSpecNotFound) {
		t.Fatalf("err=%v, want wrap of ErrSpecNotFound for absolute path", err)
	}
}

func TestFilesystemSpecStore_DoesNotValidate(t *testing.T) {
	// Parent fragments may legitimately be partial. The store must not
	// invoke Validate; only Normalize validates the merged result.
	root, err := filepath.Abs("testdata/store")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "partial.yaml"),
		[]byte("apiVersion: forge.praxis-os.dev/v0\nkind: AgentSpec\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(root, "partial.yaml")) })

	s := &FilesystemSpecStore{Root: root}
	got, err := s.Load(context.Background(), "partial.yaml")
	if err != nil {
		t.Fatalf("load partial: %v", err)
	}
	if got.Metadata.ID != "" {
		t.Fatalf("expected empty metadata.id on partial fragment, got %q", got.Metadata.ID)
	}
}
```

- [ ] **Step 3: Run (expect pass)**

Run: `go test ./spec/... -run TestFilesystemSpecStore -v`
Expected: PASS for all five tests.

---

## Task 0.5: Lint + commit task group 0

- [ ] **Step 1: Run full spec suite + race**

Run: `go test -race ./spec/... -count=1 -v`
Expected: every test PASS, no race warnings.

- [ ] **Step 2: Lint**

Run: `make lint`
Expected: zero reports.

- [ ] **Step 3: Commit**

```bash
git add spec/errors.go spec/errors_test.go spec/store.go spec/store_test.go spec/testdata/store
git commit -m "feat(spec): SpecStore interface + filesystem and map impls

Adds the injectable parent-spec loader for the Phase 2a extends-chain
resolver. Two implementations ship: FilesystemSpecStore (root-rooted,
rejects .. escapes and absolute paths) and MapSpecStore (in-memory).
Neither runs Validate — parent fragments validate only as part of the
merged Normalize result.

Also lands the broader Phase 2a sentinel set (ErrNoSpecStore,
ErrSpecNotFound, ErrExtendsInvalid, ErrLockedFieldOverride,
ErrPhaseGatedInOverlay, ErrCompositionLimit) and reshapes
spec.Errors into a struct with a sibling sentinel slice plus a
Wrap helper, so later task groups can wrap sentinels through the
existing aggregator without revisiting errors.go.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```
