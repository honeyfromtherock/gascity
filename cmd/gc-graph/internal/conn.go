// Package internal holds connection helpers for gc-graph subcommands.
package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// OpenRig opens the rig's graph.kuzu READ_ONLY. Resolves rig path via
// `gc rig path <rig>` if that subcommand exists, falling back to the
// grid-city assets convention.
func OpenRig(rig string) (*store.DB, error) {
	root, err := resolveRig(rig)
	if err != nil {
		return nil, err
	}
	p := filepath.Join(root, ".codegraph", "graph.kuzu")
	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Errorf("no graph at %s (run scripts/codegraph-index.sh first?)", p)
	}
	return store.Open(p, store.ModeReadOnly)
}

func resolveRig(rig string) (string, error) {
	// Try `gc rig path` first.
	out, err := exec.Command("gc", "rig", "path", rig).Output()
	if err == nil {
		root := strings.TrimSpace(string(out))
		if root != "" {
			return root, nil
		}
	}
	// Fall back to grid-city assets convention.
	home, _ := os.UserHomeDir()
	candidate := filepath.Join(home, "Source", "grid-city", "assets", rig)
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		// resolve symlink
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			return resolved, nil
		}
		return candidate, nil
	}
	return "", fmt.Errorf("rig %s not found (no gc rig path, no assets/%s)", rig, rig)
}
