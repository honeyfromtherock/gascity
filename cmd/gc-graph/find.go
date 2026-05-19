package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
)

// cmdFind implements `gc graph find <query> --rig <name>`. Phase 1 uses
// a Cypher CONTAINS scan because the LadybugDB FTS extension is broken
// on darwin/arm64 (see codegraph design spec §6 followups). When FTS
// becomes available, this implementation can swap to CALL QUERY_FTS_INDEX
// without changing the CLI surface.
func cmdFind(args []string) int {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	rig := fs.String("rig", "", "rig name (required)")
	kind := fs.String("kind", "Function", "Function|Method|Class|Interface|File")
	limit := fs.Int("limit", 20, "")
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *rig == "" || fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: gc graph find <query> --rig <name> [--kind Function]")
		return 2
	}
	q := fs.Arg(0)

	db, err := internal.OpenRig(*rig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = db.Close() }()
	c := db.Connect()
	defer func() { _ = c.Close() }()

	// Build a Cypher CONTAINS scan against the selected kind table.
	// Search both name and qname for substring match (case-insensitive
	// via LOWER on both sides).
	kindTable := *kind
	cypher := buildFindQuery(kindTable, *limit)
	rows, err := c.Query(cypher, map[string]any{"q": strings.ToLower(q)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer rows.Close()

	var results []map[string]any
	for rows.HasNext() {
		t, err := rows.Next()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		urn, _ := t.GetValue(0)
		name, _ := t.GetValue(1)
		fl, _ := t.GetValue(2)
		results = append(results, map[string]any{
			"urn": urn, "name": name, "file": fl,
		})
		t.Close()
	}
	if *jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"result": results,
			"meta":   map[string]any{"rig": *rig, "fidelity": "high", "backend": "cypher-contains"},
		})
	} else {
		for _, r := range results {
			fmt.Printf("%-40s %-40s %s\n", r["name"], r["urn"], r["file"])
		}
	}
	if len(results) == 0 {
		return 2 // empty + fresh (trustworthy)
	}
	return 0
}

func buildFindQuery(kind string, limit int) string {
	// File table has a `path` field instead of `name`/`qname`.
	if kind == "File" {
		return fmt.Sprintf(`
			MATCH (n:File)
			WHERE LOWER(n.path) CONTAINS $q
			RETURN n.path AS urn, n.path AS name, n.path AS file
			LIMIT %d`, limit)
	}
	return fmt.Sprintf(`
		MATCH (n:%s)
		WHERE LOWER(n.name) CONTAINS $q OR LOWER(n.qname) CONTAINS $q
		RETURN n.urn AS urn, n.name AS name, n.file AS file
		LIMIT %d`, kind, limit)
}
