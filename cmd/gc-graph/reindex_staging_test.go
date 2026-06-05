package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStagingDir(t *testing.T) {
	got := stagingDir("/root", 1234)
	want := "/root/.codegraph.staging.1234"
	if got != want {
		t.Errorf("stagingDir = %q, want %q", got, want)
	}
}

func TestCleanStaleStaging_RemovesDeadKeepsAlive(t *testing.T) {
	root := t.TempDir()
	dead := filepath.Join(root, ".codegraph.staging.111")
	aliveDir := filepath.Join(root, ".codegraph.staging.222")
	other := filepath.Join(root, ".codegraph") // must be untouched
	for _, d := range []string{dead, aliveDir, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	isAlive := func(pid int) bool { return pid == 222 }
	cleanStaleStaging(root, isAlive)

	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Errorf("dead staging .111 should be removed")
	}
	if _, err := os.Stat(aliveDir); err != nil {
		t.Errorf("alive staging .222 should be kept: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf(".codegraph must be untouched: %v", err)
	}
}

func TestCleanStaleStaging_IgnoresNonPidDirs(t *testing.T) {
	root := t.TempDir()
	bogus := filepath.Join(root, ".codegraph.staging.notapid")
	if err := os.MkdirAll(bogus, 0o755); err != nil {
		t.Fatal(err)
	}
	cleanStaleStaging(root, func(int) bool { return false })
	if _, err := os.Stat(bogus); err != nil {
		t.Errorf("non-pid staging dir should be left alone: %v", err)
	}
}

func TestPidAlive_SelfTrueZeroFalse(t *testing.T) {
	if !pidAlive(os.Getpid()) {
		t.Errorf("current process should be alive")
	}
	if pidAlive(0) {
		t.Errorf("pid 0 should be reported not-alive")
	}
}
