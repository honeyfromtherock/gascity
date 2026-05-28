package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// maxFilesPerHook caps the number of distinct files blasted per PreToolUse
// invocation. Total output bounded by maxFilesPerHook * perFileMaxTokens.
const maxFilesPerHook = 3

// perFileMaxTokens is the cap passed to `gc graph blast --max-tokens` per file.
// 800 tok shared across at most 3 files keeps the injected context below
// ~2400 tok worst case — well under the model's working memory headroom.
const perFileMaxTokens = 800

// runHookPreEdit reads Claude Code's PreToolUse JSON from stdin and runs
// blast on every touched file (capped at maxFilesPerHook). Output is appended
// to model context. Hooks NEVER block: every error path returns exit 0.
//
// Tool input shape today (Claude Code as of 2026-05):
//   - Edit/Write: tool_input.file_path is a single string
//   - MultiEdit: tool_input.file_path is a single string; tool_input.edits[]
//     contains old_string/new_string pairs but no per-edit file_path
//
// We additionally read per-edit file_path for forward-compatibility with
// hypothetical multi-file tools that may emit it.
func runHookPreEdit(_ []string) int {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		return 0
	}
	var payload struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			FilePath string `json:"file_path"`
			Edits    []struct {
				FilePath string `json:"file_path"`
			} `json:"edits"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	paths := collectUniquePaths(payload.ToolInput.FilePath, payload.ToolInput.Edits)
	if len(paths) == 0 {
		return 0
	}
	if len(paths) > maxFilesPerHook {
		paths = paths[:maxFilesPerHook]
	}
	bin := os.Getenv("GC_GRAPH_BIN")
	if bin == "" {
		bin = os.Args[0]
	}
	for _, p := range paths {
		cmd := exec.Command(bin, "blast", "--file", p, "--format", "hook", "--max-tokens", fmt.Sprintf("%d", perFileMaxTokens))
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stdout, "<gc graph hook: blast unavailable>")
		}
	}
	return 0
}

// collectUniquePaths returns up to N distinct, non-empty file paths from a
// PreToolUse payload. Preserves caller order so the top-level file_path
// (if present) blasts first.
func collectUniquePaths(topLevel string, edits []struct {
	FilePath string `json:"file_path"`
}) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	add(topLevel)
	for _, e := range edits {
		add(e.FilePath)
	}
	return out
}

// Stubs for sibling subcommands so the dispatcher compiles. Real impls in later tasks.
func runHookUserPrompt(args []string) int { return runHookUserPromptReal(args) }
func runHookList(args []string) int    { return runHookListReal(args) }
func runHookDisable(args []string) int { return runHookDisableReal(args) }
func runHookTest(args []string) int    { return runHookTestReal(args) }
