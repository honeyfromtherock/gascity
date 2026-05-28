package main

import (
	"runtime"
	"strings"
	"testing"
)

// TestGatherGraphContext_BinaryNotFound: when gc-graph is not on PATH,
// gatherGraphContext returns ("", err). Caller will log + continue.
func TestGatherGraphContext_BinaryNotFound(t *testing.T) {
	t.Setenv("GC_GRAPH_BIN", "/nonexistent-binary-xyz-99")
	out, err := gatherGraphContext("bd-fake-123", 1500)
	if err == nil {
		t.Errorf("expected error when binary not on PATH")
	}
	if out != "" {
		t.Errorf("expected empty output on missing binary, got %q", out)
	}
}

// TestGatherGraphContext_ExecFailure: when the binary exists but exits non-zero,
// gatherGraphContext returns ("", err). Tests use /usr/bin/false (always exits 1).
func TestGatherGraphContext_ExecFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /usr/bin/false")
	}
	t.Setenv("GC_GRAPH_BIN", "/usr/bin/false")
	out, err := gatherGraphContext("bd-fake-123", 1500)
	if err == nil {
		t.Errorf("expected error from /usr/bin/false")
	}
	if out != "" {
		t.Errorf("expected empty output on failure, got %q", out)
	}
}

// TestGatherGraphContext_HappyPath: when the binary succeeds and prints output,
// gatherGraphContext returns (stdout, nil). Uses /bin/echo as a stand-in to
// prove exec + capture work; echo prints all args so we just confirm the
// output is non-empty and contains the bead ID.
func TestGatherGraphContext_HappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /bin/echo")
	}
	t.Setenv("GC_GRAPH_BIN", "/bin/echo")
	out, err := gatherGraphContext("bd-fake-123", 1500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "bd-fake-123") {
		t.Errorf("expected output to mention bead ID, got %q", out)
	}
	if !strings.Contains(out, "--max-tokens") {
		t.Errorf("expected output to include --max-tokens arg, got %q", out)
	}
}

// TestGatherGraphContext_MaxTokensFormatted: --max-tokens is rendered as a
// decimal string, not the int literal. This catches strconv vs fmt slip-ups.
func TestGatherGraphContext_MaxTokensFormatted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX /bin/echo")
	}
	t.Setenv("GC_GRAPH_BIN", "/bin/echo")
	out, _ := gatherGraphContext("bd-x", 2500)
	if !strings.Contains(out, "2500") {
		t.Errorf("expected --max-tokens 2500 in output, got %q", out)
	}
}
