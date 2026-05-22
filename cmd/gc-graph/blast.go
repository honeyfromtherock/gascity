package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
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

	// Resolve URN from --file if positional urn missing.
	urn := ""
	if fs.NArg() >= 1 {
		urn = fs.Arg(0)
	}
	if urn == "" && *file != "" {
		rigName, resolved, err := resolveFileURN(*file)
		if err != nil {
			if *format == "hook" {
				fmt.Fprintln(os.Stdout, "<gc graph: file not in graph>")
				return 0
			}
			fmt.Fprintf(os.Stderr, "blast --file: %v\n", err)
			return 1
		}
		urn = resolved
		if *rig == "" {
			*rig = rigName
		}
	}

	if urn == "" || (!*allRigs && *rig == "" && *root == "") {
		if *format == "hook" {
			fmt.Fprintln(os.Stdout, "<gc graph: missing urn or rig>")
			return 0
		}
		fmt.Fprintln(os.Stderr, "usage: gc graph blast <urn>|--file <path> --rig <name> [--root <path>] [--depth N] [--all-rigs] [--format json|hook] [--max-tokens N]")
		return 2
	}
	queries := blastQueries(*depth)

	if *allRigs {
		rigs, err := LoadRigs()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		perRig := map[string]map[string][]string{}
		err = ForEachRig(rigs, func(r rigdir.Rig, alias string, conn *store.Conn) error {
			out := map[string][]string{
				"symbols": {}, "files": {}, "tests": {}, "endpoints": {}, "db_columns": {},
			}
			runBlastQueries(conn, queries, urn, out)
			perRig[r.Name] = out
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if *format == "hook" {
			merged := map[string][]string{
				"symbols": {}, "files": {}, "tests": {}, "endpoints": {}, "db_columns": {},
			}
			for _, out := range perRig {
				for k, v := range out {
					merged[k] = append(merged[k], v...)
				}
			}
			fmt.Print(emitHookFormat(urn, merged, *maxTokens))
			return 0
		}
		_ = json.NewEncoder(os.Stdout).Encode(perRig)
		return 0
	}

	db, err := internal.OpenRig(*rig, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = db.Close() }()
	c := db.Connect()
	defer func() { _ = c.Close() }()

	out := map[string][]string{
		"symbols": {}, "files": {}, "tests": {}, "endpoints": {}, "db_columns": {},
	}
	runBlastQueries(c, queries, urn, out)
	if *format == "hook" {
		fmt.Print(emitHookFormat(urn, out, *maxTokens))
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

func blastQueries(depth int) map[string]string {
	return map[string]string{
		"symbols": fmt.Sprintf(
			`MATCH (n)-[:CALLS|REFERENCES*1..%d]->(t) WHERE t.urn=$urn
				 RETURN DISTINCT n.qname LIMIT 500`, depth),
		"tests": `MATCH (t:Test)-[:TESTS|CALLS*1..4]->(s) WHERE s.urn=$urn
				   RETURN DISTINCT t.file LIMIT 200`,
		"endpoints": `MATCH (n)-[:HANDLES|CALLS_EP]->(e:Endpoint) WHERE n.urn=$urn
					   RETURN DISTINCT e.urn LIMIT 200`,
		"db_columns": `MATCH (n)-[:READS_COL|WRITES_COL]->(c:DbColumn) WHERE n.urn=$urn
						RETURN DISTINCT c.qname LIMIT 200`,
	}
}

// runBlastQueries executes each label/cypher pair against the connection,
// accumulating row values into out[label]. Per-section errors are logged to
// stderr (e.g. missing tables on a non-core rig) but never fatal.
func runBlastQueries(c *store.Conn, queries map[string]string, urn string, out map[string][]string) {
	for label, cypher := range queries {
		rows, err := c.Query(cypher, map[string]any{"urn": urn})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s: %v\n", label, err)
			continue
		}
		for rows.HasNext() {
			t, err := rows.Next()
			if err != nil {
				break
			}
			v, _ := t.GetValue(0)
			out[label] = append(out[label], fmt.Sprint(v))
			t.Close()
		}
		rows.Close()
	}
}
