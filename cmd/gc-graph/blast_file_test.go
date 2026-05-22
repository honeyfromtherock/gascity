package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestBlastFileFlagAccepted confirms --file and --format hook are recognized
// and that a missing file produces a graceful no-op (hooks must not block).
func TestBlastFileFlagAccepted(t *testing.T) {
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	r, w, _ := os.Pipe()
	os.Stdout = w

	exit := cmdBlast([]string{"--file", "/nonexistent.go", "--format", "hook", "--max-tokens", "100"})
	w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if exit != 0 {
		t.Errorf("exit = %d, want 0 (hooks must not block on missing files)", exit)
	}
	if strings.Contains(out, "flag provided but not defined") {
		t.Errorf("--file/--format hook/--max-tokens flag rejected: %s", out)
	}
}
