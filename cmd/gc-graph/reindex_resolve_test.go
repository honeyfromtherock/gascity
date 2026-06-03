package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func neverLookPath(string) (string, error) { return "", errors.New("not on PATH") }

func TestResolveIndexer_PrefersExistingBinFlag(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "gc-codegraph")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveIndexerFrom(bin, "/nonexistent/exe", neverLookPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != bin {
		t.Errorf("got %q, want %q", got, bin)
	}
}

func TestResolveIndexer_FallsBackToSiblingOfExecutable(t *testing.T) {
	dir := t.TempDir()
	sibling := filepath.Join(dir, "gc-codegraph")
	if err := os.WriteFile(sibling, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "gc-graph")
	// bin flag points at a path that does not exist → must fall back to the
	// gc-codegraph sibling of the executable (the installed-deployment case).
	got, err := resolveIndexerFrom("./bin/gc-codegraph", exe, neverLookPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != sibling {
		t.Errorf("got %q, want sibling %q", got, sibling)
	}
}

func TestResolveIndexer_FallsBackToPath(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "gc-codegraph" {
			return "/usr/local/bin/gc-codegraph", nil
		}
		return "", errors.New("not found")
	}
	// Neither the bin flag nor a sibling exists → use PATH.
	got, err := resolveIndexerFrom("/nope/gc-codegraph", "/also/nope/gc-graph", lookPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/usr/local/bin/gc-codegraph" {
		t.Errorf("got %q, want PATH result", got)
	}
}

func TestResolveIndexer_ErrorsWhenNoneFound(t *testing.T) {
	_, err := resolveIndexerFrom("/nope/gc-codegraph", "/also/nope/gc-graph", neverLookPath)
	if err == nil {
		t.Error("expected error when indexer cannot be resolved")
	}
}
