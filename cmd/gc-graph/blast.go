package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
)

// cmdBlast implements `gc graph blast <urn> --rig <name> [--depth N]`.
// Returns the blast radius bundle: symbols transitively reachable via
// CALLS/REFERENCES, tests that touch the symbol, endpoints handled or
// called, and db columns read/written.
func cmdBlast(args []string) int {
	fs := flag.NewFlagSet("blast", flag.ContinueOnError)
	rig := fs.String("rig", "", "")
	root := fs.String("root", "", "override rig root path (skips rig resolution)")
	depth := fs.Int("depth", 3, "")
	allRigs := fs.Bool("all-rigs", false, "ATTACH all rigs from ~/.codegraph/rigs.toml and query across them")
	file := fs.String("file", "", "resolve URN from this file path (alternative to positional <urn>)")
	format := fs.String("format", "json", "output format: json|hook")
	maxTokens := fs.Int("max-tokens", 0, "cap output at ~N tokens (best-effort, ~4 chars/token); 0 = no cap")
	if err := fs.Parse(args); err != nil {
		if *format == "hook" {
			fmt.Fprintln(os.Stdout, "<gc graph: bad flags>")
			return 0
		}
		return 2
	}

	urn := ""
	if fs.NArg() >= 1 {
		urn = fs.Arg(0)
	}

	// Hook-friendly --file path: graceful no-op when the file isn't under
	// any rig and the caller asked for hook output.
	if urn == "" && *file != "" {
		_, _, ferr := resolveFileURN(*file)
		if ferr != nil && *format == "hook" {
			fmt.Fprintln(os.Stdout, "<gc graph: file not in graph>")
			return 0
		}
	}

	if urn == "" && *file == "" {
		if *format == "hook" {
			fmt.Fprintln(os.Stdout, "<gc graph: missing urn or rig>")
			return 0
		}
		fmt.Fprintln(os.Stderr, "usage: gc graph blast <urn>|--file <path> --rig <name> [--root <path>] [--depth N] [--all-rigs] [--format json|hook] [--max-tokens N]")
		return 2
	}

	// Suppress unused-variable complaints for legacy CLI flags retained for
	// backward compatibility. Rig/root selection now happens inside
	// queries.Blast based on the file resolution; --all-rigs is currently
	// reduced to single-rig dispatch (queries.Blast picks one rig).
	_ = rig
	_ = root
	_ = allRigs

	rigs, err := LoadRigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	result := queries.Blast(rigs, *file, urn, *depth, 500)

	subject := urn
	if subject == "" {
		subject = result.Subject
	}

	out := map[string][]string{
		"symbols":    result.Symbols,
		"files":      result.Files,
		"tests":      result.Tests,
		"endpoints":  result.Endpoints,
		"db_columns": result.DbColumns,
	}
	// Ensure non-nil slices for stable JSON output.
	for k, v := range out {
		if v == nil {
			out[k] = []string{}
		}
	}

	if *format == "hook" {
		fmt.Print(emitHookFormat(subject, out, *maxTokens))
		return 0
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
	return 0
}

// resolveFileURN walks rigs.toml to find the rig containing path, returning
// the rig name and the URN form (rel path under rig root, matching tier2's
// File.urn convention). Errors if path is outside all rigs or the rig has no
// graph.kuzu yet.
func resolveFileURN(path string) (rigName, urn string, err error) {
	rigs, err := LoadRigs()
	if err != nil {
		return "", "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	for _, r := range rigs {
		rootAbs, _ := filepath.Abs(r.Root)
		if !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) && abs != rootAbs {
			continue
		}
		rel, _ := filepath.Rel(rootAbs, abs)
		graphPath := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
		if _, statErr := os.Stat(graphPath); statErr != nil {
			return r.Name, "", fmt.Errorf("rig %q not indexed (no %s)", r.Name, graphPath)
		}
		return r.Name, rel, nil
	}
	return "", "", fmt.Errorf("file %q not under any registered rig root", path)
}

// emitHookFormat renders a compact markdown blast-radius summary suitable for
// embedding in tool hook output. If maxTokens > 0, output is truncated at
// ~maxTokens*4 chars with a sentinel.
func emitHookFormat(urn string, out map[string][]string, maxTokens int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Blast radius: `%s`\n\n", urn)

	sections := []string{"symbols", "files", "tests", "endpoints", "db_columns"}
	any := false
	for _, sec := range sections {
		rows := out[sec]
		if len(rows) == 0 {
			continue
		}
		any = true
		fmt.Fprintf(&b, "**%s** (%d)\n", sec, len(rows))
		sorted := append([]string{}, rows...)
		sort.Strings(sorted)
		limit := len(sorted)
		if limit > 20 {
			limit = 20
		}
		for _, v := range sorted[:limit] {
			fmt.Fprintf(&b, "- %s\n", v)
		}
		if len(sorted) > limit {
			fmt.Fprintf(&b, "- _(+%d more)_\n", len(sorted)-limit)
		}
		b.WriteString("\n")
	}
	if !any {
		b.WriteString("_no callers, tests, endpoints, or db columns found_\n")
	}

	s := b.String()
	if maxTokens > 0 {
		cap := maxTokens * 4
		if len(s) > cap {
			s = s[:cap] + "\n\n_(truncated at ~" + fmt.Sprintf("%d", maxTokens) + " tokens)_\n"
		}
	}
	return s
}
