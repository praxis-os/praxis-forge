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
		0o600,
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
