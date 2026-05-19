package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
)

// cmdCallers implements `gc graph callers <urn> --rig <name> [--depth N]`.
// Returns the set of symbols that call (directly or transitively up to
// depth N) the given target URN.
func cmdCallers(args []string) int {
	fs := flag.NewFlagSet("callers", flag.ExitOnError)
	rig := fs.String("rig", "", "")
	depth := fs.Int("depth", 1, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *rig == "" || fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: gc graph callers <urn> --rig <name> [--depth N]")
		return 2
	}
	urn := fs.Arg(0)

	db, err := internal.OpenRig(*rig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = db.Close() }()
	c := db.Connect()
	defer func() { _ = c.Close() }()

	// LadybugDB v0.12.2: [:CALLS*1..N] bounded depth (no SHORTEST — parser
	// rejects the SHORTEST keyword in relationship patterns).
	cypher := fmt.Sprintf(`
		MATCH (caller)-[:CALLS*1..%d]->(target)
		WHERE target.urn = $urn
		RETURN DISTINCT caller.qname AS qname, caller.file AS file, caller.start_line AS line
		LIMIT 200`, *depth)
	rows, err := c.Query(cypher, map[string]any{"urn": urn})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer rows.Close()
	count := 0
	for rows.HasNext() {
		t, err := rows.Next()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		qn, _ := t.GetValue(0)
		fl, _ := t.GetValue(1)
		ln, _ := t.GetValue(2)
		fmt.Printf("%s\t%s:%v\n", qn, fl, ln)
		t.Close()
		count++
	}
	if count == 0 {
		return 2
	}
	return 0
}
