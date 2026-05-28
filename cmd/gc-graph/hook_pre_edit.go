package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// runHookPreEdit reads Claude Code's PreToolUse JSON from stdin and runs
// blast on the touched file. Output is appended to model context.
// Hooks NEVER block: every error path returns exit 0.
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
	path := payload.ToolInput.FilePath
	if path == "" && len(payload.ToolInput.Edits) > 0 {
		path = payload.ToolInput.Edits[0].FilePath
	}
	if path == "" {
		return 0
	}
	bin := os.Getenv("GC_GRAPH_BIN")
	if bin == "" {
		bin = os.Args[0]
	}
	cmd := exec.Command(bin, "blast", "--file", path, "--format", "hook", "--max-tokens", "800")
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stdout, "<gc graph hook: blast unavailable>")
	}
	return 0
}

// Stubs for sibling subcommands so the dispatcher compiles. Real impls in later tasks.
func runHookUserPrompt(args []string) int { return runHookUserPromptReal(args) }
func runHookList(args []string) int    { return runHookListReal(args) }
func runHookDisable(args []string) int { return runHookDisableReal(args) }
func runHookTest(args []string) int    { return runHookTestReal(args) }
