// Package discover inspects a rig's working directory and returns what
// languages are present and what paths must be excluded from indexing.
package discover

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HardExcludes are always excluded regardless of .codegraphignore.
var HardExcludes = []string{
	".worktrees/", "node_modules/", "vendor/", "dist/",
	".next/", "build/", ".encore/", ".git/",
}

// Rig holds the discovered state of a rig's working directory.
type Rig struct {
	Root     string
	Langs    []string
	excludes []string // prefixes (slash-terminated for dirs, exact for files)
}

// Discover inspects root and returns detected languages and exclusion rules.
func Discover(root string) (*Rig, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("rig root %s: %w", abs, err)
	}

	r := &Rig{Root: abs, excludes: append([]string{}, HardExcludes...)}

	// Per-rig overrides
	if extra, err := readIgnore(filepath.Join(abs, ".codegraphignore")); err == nil {
		r.excludes = append(r.excludes, extra...)
	}

	// Language detection: presence of marker files anywhere not-excluded.
	r.Langs = detectLangs(r)
	return r, nil
}

// IsExcluded reports whether relPath (relative to Root) should be skipped.
func (r *Rig) IsExcluded(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	for _, ex := range r.excludes {
		if strings.HasSuffix(ex, "/") {
			if strings.HasPrefix(relPath, ex) || strings.Contains(relPath, "/"+ex) {
				return true
			}
		} else if relPath == ex {
			return true
		}
	}
	return false
}

func detectLangs(r *Rig) []string {
	want := map[string]string{
		"go.mod":        "go",
		"tsconfig.json": "typescript",
		"atlas.hcl":     "sql",
	}
	pyMarker := false
	found := map[string]bool{}

	_ = filepath.WalkDir(r.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(r.Root, path)
		if r.IsExcluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			base := filepath.Base(path)
			if lang, ok := want[base]; ok {
				found[lang] = true
			}
			if strings.HasSuffix(base, ".py") {
				pyMarker = true
			}
		}
		return nil
	})

	out := []string{}
	for lang := range found {
		out = append(out, lang)
	}
	if pyMarker {
		out = append(out, "python")
	}
	return out
}

func readIgnore(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		t := strings.TrimSpace(s.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		lines = append(lines, t)
	}
	return lines, s.Err()
}
