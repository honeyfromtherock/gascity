package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
)

const urnCapPerPrompt = 3

var endpointURNRe = regexp.MustCompile(`endpoint:[a-z][a-zA-Z0-9_]*\.[A-Z][A-Za-z0-9_]*`)

// runHookUserPromptReal reads Claude Code's UserPromptSubmit JSON from stdin,
// extracts endpoint URN patterns, and runs endpoint-consumers per match.
// Output appended to context. Hooks never block — every error path returns 0.
func runHookUserPromptReal(_ []string) int {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	urns := extractEndpointURNs(payload)
	bin := os.Getenv("GC_GRAPH_BIN")
	if bin == "" {
		bin = os.Args[0]
	}
	for _, urn := range urns {
		cmd := exec.Command(bin, "endpoint-consumers", "--urn", urn)
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stdout, "<gc graph hook: endpoint-consumers %s unavailable>\n", urn)
		}
	}
	return 0
}

// extractEndpointURNs scans the prompt for endpoint URN patterns, dedupes,
// caps at urnCapPerPrompt to bound work.
func extractEndpointURNs(payload map[string]any) []string {
	prompt, _ := payload["prompt"].(string)
	if prompt == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range endpointURNRe.FindAllString(prompt, -1) {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
		if len(out) >= urnCapPerPrompt {
			break
		}
	}
	return out
}
