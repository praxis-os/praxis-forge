# Task 05 — Manifest extensions

Extend [`manifest/manifest.go`](../../../manifest/manifest.go) with the two Phase 3 fields: `Manifest.ExpandedHash` and `ResolvedComponent.InjectedBySkill`. Tests verify round-trip and `omitempty` behavior for both.

## Files

- Modify: [`manifest/manifest.go`](../../../manifest/manifest.go)
- Modify: [`manifest/manifest_test.go`](../../../manifest/manifest_test.go)

## Background

Current Manifest shape ([`manifest/manifest.go:11-20`](../../../manifest/manifest.go#L11-L20)):

```go
type Manifest struct {
    SpecID         string               `json:"specId"`
    SpecVersion    string               `json:"specVersion"`
    BuiltAt        time.Time            `json:"builtAt"`
    NormalizedHash string               `json:"normalizedHash"`
    Capabilities   Capabilities         `json:"capabilities"`
    ExtendsChain   []string             `json:"extendsChain,omitempty"`
    Overlays       []OverlayAttribution `json:"overlays,omitempty"`
    Resolved       []ResolvedComponent  `json:"resolved"`
}

type ResolvedComponent struct {
    Kind        string         `json:"kind"`
    ID          string         `json:"id"`
    Config      map[string]any `json:"config,omitempty"`
    Descriptors any            `json:"descriptors,omitempty"`
}
```

Design spec §"Manifest additions" adds:

- `Manifest.ExpandedHash string` with `omitempty` — emitted iff `spec.skills[]` was non-empty.
- `ResolvedComponent.InjectedBySkill string` with `omitempty` — set when a skill drove inclusion of that component.

## Steps

- [ ] **Step 1: Write failing tests**

Append to [`manifest/manifest_test.go`](../../../manifest/manifest_test.go):

```go
func TestManifest_ExpandedHashRoundTrip(t *testing.T) {
	m := Manifest{
		SpecID:         "acme.demo",
		SpecVersion:    "0.1.0",
		BuiltAt:        time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		NormalizedHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpandedHash:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Resolved: []ResolvedComponent{
			{Kind: "provider", ID: "provider.fake@1.0.0"},
		},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"expandedHash":"bbbb`) {
		t.Errorf("expandedHash not present in JSON: %s", out)
	}

	var got Manifest
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.ExpandedHash != m.ExpandedHash {
		t.Errorf("ExpandedHash: want %q, got %q", m.ExpandedHash, got.ExpandedHash)
	}
}

func TestManifest_ExpandedHashOmittedWhenEmpty(t *testing.T) {
	m := Manifest{
		SpecID:         "acme.demo",
		SpecVersion:    "0.1.0",
		BuiltAt:        time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		NormalizedHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		// ExpandedHash intentionally empty
		Resolved: []ResolvedComponent{{Kind: "provider", ID: "provider.fake@1.0.0"}},
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "expandedHash") {
		t.Errorf("expandedHash should be omitted when empty: %s", out)
	}
}

func TestResolvedComponent_InjectedBySkillRoundTrip(t *testing.T) {
	rc := ResolvedComponent{
		Kind:            "tool_pack",
		ID:              "toolpack.http-get@1.0.0",
		InjectedBySkill: "skill.structured-output@1.0.0",
	}
	out, err := json.Marshal(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"injectedBySkill":"skill.structured-output@1.0.0"`) {
		t.Errorf("injectedBySkill missing: %s", out)
	}

	var got ResolvedComponent
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.InjectedBySkill != rc.InjectedBySkill {
		t.Errorf("InjectedBySkill: want %q, got %q", rc.InjectedBySkill, got.InjectedBySkill)
	}
}

func TestResolvedComponent_InjectedBySkillOmittedWhenEmpty(t *testing.T) {
	rc := ResolvedComponent{
		Kind: "provider",
		ID:   "provider.fake@1.0.0",
	}
	out, err := json.Marshal(rc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "injectedBySkill") {
		t.Errorf("injectedBySkill should be omitted when empty: %s", out)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./manifest/ -run 'TestManifest_ExpandedHash|TestResolvedComponent_InjectedBySkill' -v`

Expected: FAIL — unknown fields `ExpandedHash` and `InjectedBySkill`.

- [ ] **Step 3: Extend the types**

Edit [`manifest/manifest.go`](../../../manifest/manifest.go). Replace the `Manifest` and `ResolvedComponent` structs with:

```go
// Manifest is the build record returned alongside every BuiltAgent. It
// is JSON-serializable so callers can persist it for audit, diff, and
// inspection workflows.
type Manifest struct {
	SpecID         string               `json:"specId"`
	SpecVersion    string               `json:"specVersion"`
	BuiltAt        time.Time            `json:"builtAt"`
	NormalizedHash string               `json:"normalizedHash"`
	// ExpandedHash is the SHA-256 of the canonical JSON of the
	// post-skill-expansion AgentSpec. Emitted when spec.skills[] was
	// non-empty (Phase 3). Omitted when no skill expansion ran.
	ExpandedHash   string               `json:"expandedHash,omitempty"`
	Capabilities   Capabilities         `json:"capabilities"`
	ExtendsChain   []string             `json:"extendsChain,omitempty"`
	Overlays       []OverlayAttribution `json:"overlays,omitempty"`
	Resolved       []ResolvedComponent  `json:"resolved"`
}

// OverlayAttribution identifies one overlay that contributed to the
// build. Mirror of spec.OverlayAttribution; duplicated here so the
// manifest package keeps zero internal dependencies.
type OverlayAttribution struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

type ResolvedComponent struct {
	Kind        string         `json:"kind"`
	ID          string         `json:"id"`
	Config      map[string]any `json:"config,omitempty"`
	Descriptors any            `json:"descriptors,omitempty"`
	// InjectedBySkill is the skill id that drove inclusion of this
	// component via Phase 3 expansion. Empty for user-declared or
	// for the skills themselves (Kind == "skill").
	InjectedBySkill string `json:"injectedBySkill,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./manifest/ -run 'TestManifest_ExpandedHash|TestResolvedComponent_InjectedBySkill' -v`

Expected: PASS (4 new tests).

- [ ] **Step 5: Run full manifest suite**

Run: `go test ./manifest/... -v && go vet ./manifest/...`

Expected: all tests still pass; no vet complaints.

- [ ] **Step 6: Commit**

```bash
git add manifest/manifest.go manifest/manifest_test.go
git commit -m "$(cat <<'EOF'
feat(manifest): ExpandedHash + InjectedBySkill fields

Manifest.ExpandedHash is the hash of the post-skill-expansion AgentSpec;
omitted when no skills ran. ResolvedComponent.InjectedBySkill records
which skill drove inclusion; empty for user-declared refs. Both use
omitempty so manifests produced by Phase-1/Phase-2 builds (no Phase-3
content) serialize identically to before.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

## Expected state after this task

- `manifest/manifest.go`: `Manifest.ExpandedHash` + `ResolvedComponent.InjectedBySkill` added with `omitempty`.
- `manifest/manifest_test.go`: 4 new tests (round-trip + omitempty for both fields).
- All pre-existing manifest tests pass.
- One commit added.
