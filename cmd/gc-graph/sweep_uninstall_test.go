package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSweepPlistPathEndsCorrectly(t *testing.T) {
	p, err := sweepPlistPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, "/Library/LaunchAgents/com.gridbase.codegraph.sweep.plist") {
		t.Errorf("sweepPlistPath = %q, want it to end with /Library/LaunchAgents/com.gridbase.codegraph.sweep.plist", p)
	}
}

func TestRemoveIfExists_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thing.plist")

	// Removing a non-existent file is not an error.
	if err := removeIfExists(path); err != nil {
		t.Errorf("removeIfExists on absent file: %v", err)
	}

	// Removing an existing file removes it and returns nil.
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeIfExists(path); err != nil {
		t.Errorf("removeIfExists on present file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after removeIfExists")
	}
}
