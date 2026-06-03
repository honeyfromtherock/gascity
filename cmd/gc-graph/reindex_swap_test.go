package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapStagedGraph_ReplacesGraphPreservingRuntimeFiles(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, ".codegraph")
	staging := filepath.Join(root, ".codegraph.staging")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	// Old graph + a runtime file that MUST survive the swap.
	if err := os.WriteFile(filepath.Join(out, "graph.kuzu"), []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, ".last-trigger"), []byte("999"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Fresh staged graph + manifest.
	if err := os.WriteFile(filepath.Join(staging, "graph.kuzu"), []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := swapStagedGraph(out, staging); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(filepath.Join(out, "graph.kuzu"))
	if string(b) != "NEW" {
		t.Errorf("graph not swapped: got %q want NEW", b)
	}
	lt, _ := os.ReadFile(filepath.Join(out, ".last-trigger"))
	if string(lt) != "999" {
		t.Errorf(".last-trigger not preserved: got %q want 999", lt)
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); err != nil {
		t.Errorf("manifest.json not refreshed: %v", err)
	}
}

func TestSwapStagedGraph_ErrorsAndPreservesOldWhenStagedMissing(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, ".codegraph")
	staging := filepath.Join(root, ".codegraph.staging")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "graph.kuzu"), []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	// staging has no graph.kuzu → swap must error and leave the old graph intact.
	if err := swapStagedGraph(out, staging); err == nil {
		t.Error("expected error when staged graph.kuzu is missing")
	}
	b, _ := os.ReadFile(filepath.Join(out, "graph.kuzu"))
	if string(b) != "OLD" {
		t.Errorf("old graph disturbed on failed swap: got %q want OLD", b)
	}
}
