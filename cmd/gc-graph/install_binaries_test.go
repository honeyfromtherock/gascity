package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallBinaries_CopiesBoth(t *testing.T) {
	src := t.TempDir()
	// Fake source binaries.
	selfPath := filepath.Join(src, "gc-graph")
	indexerPath := filepath.Join(src, "gc-codegraph")
	if err := os.WriteFile(selfPath, []byte("SELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexerPath, []byte("INDEXER"), 0o755); err != nil {
		t.Fatal(err)
	}
	bindir := filepath.Join(t.TempDir(), "bin")

	if err := installBinaries(bindir, selfPath, indexerPath); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{"gc-graph": "SELF", "gc-codegraph": "INDEXER"} {
		b, err := os.ReadFile(filepath.Join(bindir, name))
		if err != nil {
			t.Fatalf("%s not installed: %v", name, err)
		}
		if string(b) != want {
			t.Errorf("%s content = %q, want %q", name, b, want)
		}
		info, _ := os.Stat(filepath.Join(bindir, name))
		if info.Mode()&0o111 == 0 {
			t.Errorf("%s not executable", name)
		}
	}
}

func TestInstallBinaries_NoOpWhenSrcEqualsDst(t *testing.T) {
	bindir := t.TempDir()
	selfPath := filepath.Join(bindir, "gc-graph")
	indexerPath := filepath.Join(bindir, "gc-codegraph")
	if err := os.WriteFile(selfPath, []byte("SELF"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexerPath, []byte("INDEXER"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Installing into the same dir the binaries already live in must not error
	// or truncate them.
	if err := installBinaries(bindir, selfPath, indexerPath); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(selfPath)
	if string(b) != "SELF" {
		t.Errorf("self binary corrupted: %q", b)
	}
}
