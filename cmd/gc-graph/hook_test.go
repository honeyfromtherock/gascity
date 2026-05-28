package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestHookPreEditWithMissingFile confirms the pre-edit hook prints nothing
// (or a single sentinel) when the file isn't in the graph, AND exits 0.
// Hooks must NEVER block an agent action.
func TestHookPreEditWithMissingFile(t *testing.T) {
	stdinJSON := `{"tool_name": "Edit", "tool_input": {"file_path": "/nonexistent/file.go"}}`
	out, exit := runHookWithStdin(t, "pre-edit", stdinJSON)
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if strings.Contains(out, "panic") || strings.Contains(out, "Error:") {
		t.Errorf("hook output contains panic/Error: %s", out)
	}
}

func TestHookPreEditWithMalformedStdin(t *testing.T) {
	out, exit := runHookWithStdin(t, "pre-edit", `not valid json`)
	if exit != 0 {
		t.Errorf("exit = %d, want 0 (hooks never block)", exit)
	}
	_ = out
}

func TestHookPreEditWithMultiEdit(t *testing.T) {
	stdinJSON := `{"tool_name": "MultiEdit", "tool_input": {"edits": [{"file_path": "/foo.go"}]}}`
	_, exit := runHookWithStdin(t, "pre-edit", stdinJSON)
	if exit != 0 {
		t.Errorf("MultiEdit edits[0].file_path extraction failed; exit = %d", exit)
	}
}

// runHookWithStdin invokes the hook subcommand with stdin piped in.
func runHookWithStdin(t *testing.T, subcommand, stdinJSON string) (string, int) {
	t.Helper()
	// Point the pre-edit hook at a no-op binary so tests don't re-exec the
	// test binary (which would recurse and hang).
	t.Setenv("GC_GRAPH_BIN", "/usr/bin/true")
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() { os.Stdin = oldStdin; os.Stdout = oldStdout }()

	stdinR, stdinW, _ := os.Pipe()
	os.Stdin = stdinR
	go func() {
		_, _ = stdinW.Write([]byte(stdinJSON))
		stdinW.Close()
	}()

	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	exit := runHook([]string{subcommand})
	stdoutW.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stdoutR)
	return buf.String(), exit
}
