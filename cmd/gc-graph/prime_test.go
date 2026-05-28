package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrimeWithExplicitInputsWritesMarkdown(t *testing.T) {
	// Prevent test-binary self-recursion when prime shells out via os.Args[0].
	t.Setenv("GC_GRAPH_BIN", "/usr/bin/true")
	dir := t.TempDir()
	out := filepath.Join(dir, "graph-prime.md")
	exit := runPrime([]string{
		"--touched-file", "/Users/andrewthompson/Source/gridbase/core/backend/auth/auth.go",
		"--endpoint-urn", "endpoint:auth.Login",
		"--out", out,
	})
	if exit != 0 {
		t.Fatalf("exit = %d, want 0", exit)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output file not written: %v", err)
	}
	if !strings.Contains(string(b), "# Codegraph context") {
		t.Errorf("output missing header: %s", string(b))
	}
}

func TestPrimeMissingInputsIsNoOp(t *testing.T) {
	t.Setenv("GC_GRAPH_BIN", "/usr/bin/true")
	dir := t.TempDir()
	out := filepath.Join(dir, "graph-prime.md")
	exit := runPrime([]string{"--out", out})
	if exit != 0 {
		t.Errorf("exit = %d, want 0 (no inputs = clean no-op)", exit)
	}
}
