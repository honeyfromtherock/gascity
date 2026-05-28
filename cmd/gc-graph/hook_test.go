package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

func TestHookUserPromptDetectsEndpointURN(t *testing.T) {
	stdinJSON := `{"prompt": "Refactor endpoint:auth.Login to handle 2FA"}`
	out, exit := runHookWithStdin(t, "user-prompt", stdinJSON)
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	// Output may be empty if no rigs are registered; should NOT contain panic
	if strings.Contains(out, "panic") {
		t.Fatalf("hook output contained panic: %s", out)
	}
	_ = out
}

func TestHookUserPromptIgnoresPromptsWithoutURNs(t *testing.T) {
	stdinJSON := `{"prompt": "Hello, world"}`
	out, exit := runHookWithStdin(t, "user-prompt", stdinJSON)
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for prompt without URN, got: %q", out)
	}
}

// Ensures the per-prompt URN match cap (3) is respected.
func TestHookUserPromptCapsURNsPerPrompt(t *testing.T) {
	urns := extractEndpointURNs(map[string]any{
		"prompt": "endpoint:a.A endpoint:b.B endpoint:c.C endpoint:d.D endpoint:e.E",
	})
	if len(urns) > 3 {
		t.Errorf("got %d URNs, want <= 3 (cap is 3)", len(urns))
	}
	if len(urns) != 3 {
		t.Errorf("got %d URNs, expected exactly 3", len(urns))
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

func TestHookListPrintsConfiguredHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	if err := mergeHooksIntoSettings(settingsPath); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() int {
		return listHooksFromPath(settingsPath)
	})
	if !strings.Contains(out, "PreToolUse") || !strings.Contains(out, "UserPromptSubmit") {
		t.Errorf("hook list missing expected events: %s", out)
	}
}

func TestHookDisableRemovesEntry(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	mergeHooksIntoSettings(settingsPath)
	if err := disableHookInPath(settingsPath, "PreToolUse"); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() int { return listHooksFromPath(settingsPath) })
	if strings.Contains(out, "PreToolUse") {
		t.Errorf("PreToolUse should be removed; still present: %s", out)
	}
	if !strings.Contains(out, "UserPromptSubmit") {
		t.Errorf("UserPromptSubmit should remain")
	}
}

// captureStdout runs fn with stdout redirected, returns captured output.
func captureStdout(t *testing.T, fn func() int) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
