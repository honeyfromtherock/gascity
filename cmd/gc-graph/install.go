package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/agents"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

const (
	doctrineStartSentinel = "<!-- gc-graph:doctrine start -->"
	doctrineEndSentinel   = "<!-- gc-graph:doctrine end -->"
)

func runInstall(args []string) int {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	rigName := fs.String("rig", "", "install into a specific registered rig's .claude/ directory")
	global := fs.Bool("global", false, "install into ~/.claude/")
	upgrade := fs.Bool("upgrade", false, "re-install over existing artifacts (idempotent regardless of this flag)")
	_ = upgrade // flag accepted for forward-compat; install is always idempotent
	_ = fs.Parse(args)

	if !*global && *rigName == "" {
		fmt.Fprintln(os.Stderr, "usage: gc graph install [--global | --rig <name>]")
		return 2
	}

	var targetClaudeDir, targetCLAUDEmd string
	if *global {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "user home dir: %v\n", err)
			return 1
		}
		targetClaudeDir = filepath.Join(home, ".claude")
		targetCLAUDEmd = "" // no CLAUDE.md doctrine in global install (per-rig only)
	} else {
		rigsPath, _ := rigdir.DefaultPath()
		rigs, err := rigdir.Load(rigsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rigdir: %v\n", err)
			return 1
		}
		r, err := rigdir.Lookup(rigs, *rigName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		targetClaudeDir = filepath.Join(r.Root, ".claude")
		targetCLAUDEmd = filepath.Join(r.Root, "CLAUDE.md")
	}

	if err := writeSkillFile(filepath.Join(targetClaudeDir, "skills")); err != nil {
		fmt.Fprintf(os.Stderr, "skill file: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "+ %s/skills/gc-graph.md\n", targetClaudeDir)

	settingsPath := filepath.Join(targetClaudeDir, "settings.json")
	if err := mergeHooksIntoSettings(settingsPath); err != nil {
		fmt.Fprintf(os.Stderr, "settings.json: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "+ %s (hooks merged)\n", settingsPath)

	if targetCLAUDEmd != "" {
		if err := appendDoctrine(targetCLAUDEmd); err != nil {
			fmt.Fprintf(os.Stderr, "CLAUDE.md doctrine: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "+ doctrine appended to %s\n", targetCLAUDEmd)
	}
	return 0
}

// writeSkillFile writes the embedded skill.md content to <dir>/gc-graph.md.
// Overwrites any existing file (the canonical source is the source of truth).
func writeSkillFile(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "gc-graph.md"), agents.SkillMD(), 0o644)
}

// mergeHooksIntoSettings merges the canonical hook block into <settingsPath>.
// If the file doesn't exist, it's created from the canonical block. If it
// exists, our hook entries are merged keyed by hook event name, replacing
// any existing entry with the same key but preserving entries for other
// events (e.g., user's SessionStart hooks).
//
// Backs up the original to settings.json.bak.<unix-ts> before writing.
func mergeHooksIntoSettings(settingsPath string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	var canonical map[string]any
	if err := json.Unmarshal(agents.HooksJSON(), &canonical); err != nil {
		return fmt.Errorf("parse canonical hooks: %w", err)
	}

	current := map[string]any{}
	if existing, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(existing, &current); err != nil {
			return fmt.Errorf("parse existing settings: %w", err)
		}
		bakPath := fmt.Sprintf("%s.bak.%d", settingsPath, time.Now().Unix())
		_ = os.WriteFile(bakPath, existing, 0o644)
	}

	currentHooks, _ := current["hooks"].(map[string]any)
	if currentHooks == nil {
		currentHooks = map[string]any{}
	}
	canonicalHooks, _ := canonical["hooks"].(map[string]any)
	for event, entries := range canonicalHooks {
		currentHooks[event] = entries
	}
	current["hooks"] = currentHooks

	out, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, out, 0o644)
}

// appendDoctrine appends the canonical doctrine block to <mdPath>. If the
// file already contains a doctrine block (sentinel-fenced), replaces it
// in place; otherwise appends at the end.
func appendDoctrine(mdPath string) error {
	existing := []byte("")
	if b, err := os.ReadFile(mdPath); err == nil {
		existing = b
	}
	doctrine := string(agents.DoctrineMD())
	body := string(existing)
	startIdx := strings.Index(body, doctrineStartSentinel)
	endIdx := strings.Index(body, doctrineEndSentinel)
	if startIdx >= 0 && endIdx > startIdx {
		before := body[:startIdx]
		after := body[endIdx+len(doctrineEndSentinel):]
		// doctrine already ends with "\n"; strip a leading "\n" from
		// after to avoid accumulating blank lines on repeated runs.
		after = strings.TrimLeft(after, "\n")
		newBody := before + doctrine + after
		return os.WriteFile(mdPath, []byte(newBody), 0o644)
	}
	if len(body) > 0 && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += "\n" + doctrine
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return os.WriteFile(mdPath, []byte(body), 0o644)
}
