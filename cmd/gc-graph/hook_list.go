package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func runHookListReal(_ []string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return listHooksFromPath(filepath.Join(home, ".claude", "settings.json"))
}

func listHooksFromPath(settingsPath string) int {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no settings file at %s\n", settingsPath)
		return 0
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "settings.json malformed: %v\n", err)
		return 1
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if len(hooks) == 0 {
		fmt.Fprintln(os.Stdout, "(no hooks configured)")
		return 0
	}
	fmt.Fprintf(os.Stdout, "Hooks configured in %s:\n", settingsPath)
	for event, entries := range hooks {
		fmt.Fprintf(os.Stdout, "  %s: %d entries\n", event, jsonLen(entries))
	}
	return 0
}

func jsonLen(v any) int {
	if arr, ok := v.([]any); ok {
		return len(arr)
	}
	return 0
}
