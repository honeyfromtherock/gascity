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

// defaultEndpointURNPattern matches the canonical Encore-style URN form
// "endpoint:<service>.<Method>". Used when no rig in rigs.toml supplies
// an explicit urn_pattern.
const defaultEndpointURNPattern = `endpoint:[a-z][a-zA-Z0-9_]*\.[A-Z][A-Za-z0-9_]*`

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
	urns := extractEndpointURNs(payload, loadURNPatterns())
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

// loadURNPatterns reads rigs.toml and returns the union of distinct urn_pattern
// values across all registered rigs, plus the default pattern as a fallback.
// Rigs without an explicit urn_pattern contribute the default. Invalid patterns
// are skipped silently — hooks never block on bad config.
func loadURNPatterns() []*regexp.Regexp {
	patterns := []string{defaultEndpointURNPattern}
	seen := map[string]bool{defaultEndpointURNPattern: true}
	if rigs, err := LoadRigs(); err == nil {
		for _, r := range rigs {
			if r.URNPattern == "" || seen[r.URNPattern] {
				continue
			}
			seen[r.URNPattern] = true
			patterns = append(patterns, r.URNPattern)
		}
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			out = append(out, re)
		}
	}
	return out
}

// extractEndpointURNs scans the prompt against every pattern, dedupes matches,
// caps at urnCapPerPrompt to bound work.
func extractEndpointURNs(payload map[string]any, patterns []*regexp.Regexp) []string {
	prompt, _ := payload["prompt"].(string)
	if prompt == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, re := range patterns {
		for _, m := range re.FindAllString(prompt, -1) {
			if seen[m] {
				continue
			}
			seen[m] = true
			out = append(out, m)
			if len(out) >= urnCapPerPrompt {
				return out
			}
		}
	}
	return out
}
