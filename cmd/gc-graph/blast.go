package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
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
	allRigs := fs.Bool("all-rigs", false, "ATTACH all rigs from ~/.codegraph/rigs.toml and query across them")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || (!*allRigs && *rig == "" && *root == "") {
		fmt.Fprintln(os.Stderr, "usage: gc graph blast <urn> --rig <name> [--root <path>] [--depth N] [--all-rigs]")
		return 2
	}
	urn := fs.Arg(0)
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
	_ = json.NewEncoder(os.Stdout).Encode(out)
	return 0
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
