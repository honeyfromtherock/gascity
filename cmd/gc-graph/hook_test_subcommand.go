package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// runHookTestReal dry-runs a hook with synthetic stdin to show what it would inject.
func runHookTestReal(args []string) int {
	fs := flag.NewFlagSet("hook test", flag.ExitOnError)
	tool := fs.String("tool", "Edit", "tool name (Edit, Write, MultiEdit)")
	file := fs.String("file", "", "file path to test against")
	prompt := fs.String("prompt", "", "test against user-prompt hook with this prompt")
	_ = fs.Parse(args)

	bin := os.Getenv("GC_GRAPH_BIN")
	if bin == "" {
		bin = os.Args[0]
	}
	if *prompt != "" {
		stdin := mustJSON(map[string]any{"prompt": *prompt})
		return execHookWithStdin(bin, "user-prompt", stdin)
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: gc graph hook test --tool Edit --file <p> | --prompt <text>")
		return 2
	}
	stdin := mustJSON(map[string]any{
		"tool_name":  *tool,
		"tool_input": map[string]any{"file_path": *file},
	})
	return execHookWithStdin(bin, "pre-edit", stdin)
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func execHookWithStdin(bin, subcommand, stdinJSON string) int {
	cmd := exec.Command(bin, "hook", subcommand)
	cmd.Stdin = bytes.NewBufferString(stdinJSON)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	return 0
}

// io is imported but may be referenced from sibling files; keep import explicit.
var _ = io.Discard
