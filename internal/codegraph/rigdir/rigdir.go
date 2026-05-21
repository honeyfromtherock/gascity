// Package rigdir loads and validates ~/.codegraph/rigs.toml.
package rigdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Rig is a single registered rig.
type Rig struct {
	Name    string `toml:"name"`
	Root    string `toml:"root"`
	Tier    string `toml:"tier"`              // "scip" or "endpoint"
	Profile string `toml:"profile,omitempty"` // optional profile name (e.g. "core")
}

type fileShape struct {
	Rig []Rig `toml:"rig"`
}

// DefaultPath returns ~/.codegraph/rigs.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codegraph", "rigs.toml"), nil
}

// Load parses and validates a rigs.toml file.
func Load(path string) ([]Rig, error) {
	var doc fileShape
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		return nil, fmt.Errorf("rigdir: parse %s: %w", path, err)
	}
	seen := map[string]bool{}
	for i := range doc.Rig {
		r := &doc.Rig[i]
		if r.Name == "" {
			return nil, fmt.Errorf("rigdir: rig %d missing name", i)
		}
		if r.Root == "" {
			return nil, fmt.Errorf("rigdir: rig %q missing root", r.Name)
		}
		if r.Tier != "scip" && r.Tier != "endpoint" {
			return nil, fmt.Errorf("rigdir: rig %q invalid tier %q (want scip or endpoint)", r.Name, r.Tier)
		}
		if seen[r.Name] {
			return nil, fmt.Errorf("rigdir: duplicate rig name %q", r.Name)
		}
		seen[r.Name] = true
		r.Root = expandTilde(r.Root)
	}
	return doc.Rig, nil
}

// Lookup returns the rig with the given name, or an error.
func Lookup(rigs []Rig, name string) (Rig, error) {
	for _, r := range rigs {
		if r.Name == name {
			return r, nil
		}
	}
	return Rig{}, fmt.Errorf("rigdir: no rig named %q", name)
}

func expandTilde(p string) string {
	if !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
