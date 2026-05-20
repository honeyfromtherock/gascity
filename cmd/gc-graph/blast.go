package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
)

// cmdBlast implements `gc graph blast <urn> --rig <name> [--depth N]`.
// Returns the blast radius bundle: symbols transitively reachable via
// CALLS/REFERENCES, tests that touch the symbol, endpoints handled or
// called, and db columns read/written.
func cmdBlast(args []string) int {
	fs := flag.NewFlagSet("blast", flag.ExitOnError)
	rig := fs.String("rig", "", "")
	root := fs.String("root", "", "override rig root path (skips rig resolution)")
	depth := fs.Int("depth", 3, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if (*rig == "" && *root == "") || fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: gc graph blast <urn> --rig <name> [--root <path>] [--depth N]")
		return 2
	}
	urn := fs.Arg(0)
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

	queries := map[string]string{
		"symbols": fmt.Sprintf(
			`MATCH (n)-[:CALLS|REFERENCES*1..%d]->(t) WHERE t.urn=$urn
			 RETURN DISTINCT n.qname LIMIT 500`, *depth),
		"tests": `MATCH (t:Test)-[:TESTS|CALLS*1..4]->(s) WHERE s.urn=$urn
				   RETURN DISTINCT t.file LIMIT 200`,
		"endpoints": `MATCH (n)-[:HANDLES|CALLS_EP]->(e:Endpoint) WHERE n.urn=$urn
					   RETURN DISTINCT e.urn LIMIT 200`,
		"db_columns": `MATCH (n)-[:READS_COL|WRITES_COL]->(c:DbColumn) WHERE n.urn=$urn
						RETURN DISTINCT c.qname LIMIT 200`,
	}
	for label, cypher := range queries {
		rows, err := c.Query(cypher, map[string]any{"urn": urn})
		if err != nil {
			// Per-section failure isn't fatal; the table may not be present
			// (e.g., db_columns on a non-core rig). Log to stderr.
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
	_ = json.NewEncoder(os.Stdout).Encode(out)
	return 0
}
