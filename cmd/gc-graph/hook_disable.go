package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func runHookDisableReal(args []string) int {
	fs := flag.NewFlagSet("hook disable", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: gc graph hook disable <event>")
		return 2
	}
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := disableHookInPath(settingsPath, fs.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "disabled %s in %s\n", fs.Arg(0), settingsPath)
	return 0
}

func disableHookInPath(settingsPath, event string) error {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return err
	}
	if hooks, ok := doc["hooks"].(map[string]any); ok {
		delete(hooks, event)
		doc["hooks"] = hooks
	}
	out, _ := json.MarshalIndent(doc, "", "  ")
	return os.WriteFile(settingsPath, out, 0o644)
}
