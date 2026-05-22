package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// cmdCallers implements `gc graph callers <urn> --rig <name> [--depth N]`.
// Returns the set of symbols that call (directly or transitively up to
// depth N) the given target URN.
func cmdCallers(args []string) int {
	fs := flag.NewFlagSet("callers", flag.ExitOnError)
	rig := fs.String("rig", "", "")
	root := fs.String("root", "", "override rig root path (skips rig resolution)")
	depth := fs.Int("depth", 1, "")
	allRigs := fs.Bool("all-rigs", false, "ATTACH all rigs from ~/.codegraph/rigs.toml and query across them")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || (!*allRigs && *rig == "" && *root == "") {
		fmt.Fprintln(os.Stderr, "usage: gc graph callers <urn> --rig <name> [--root <path>] [--depth N] [--all-rigs]")
		return 2
	}
	urn := fs.Arg(0)

	cypher := fmt.Sprintf(`
			MATCH (caller)-[:CALLS*1..%d]->(target)
			WHERE target.urn = $urn
			RETURN DISTINCT caller.qname AS qname, caller.file AS file, caller.start_line AS line
			LIMIT 200`, *depth)

	if *allRigs {
		rigs, err := LoadRigs()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		total := 0
		err = ForEachRig(rigs, func(r rigdir.Rig, alias string, conn *store.Conn) error {
			rows, err := conn.Query(cypher, map[string]any{"urn": urn})
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] query: %v\n", r.Name, err)
				return nil
			}
			defer rows.Close()
			printed := false
			for rows.HasNext() {
				t, err := rows.Next()
				if err != nil {
					break
				}
				qn, _ := t.GetValue(0)
				fl, _ := t.GetValue(1)
				ln, _ := t.GetValue(2)
				if !printed {
					fmt.Printf("[rig=%s]\n", r.Name)
					printed = true
				}
				fmt.Printf("%s\t%s:%v\n", qn, fl, ln)
				t.Close()
				total++
			}
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if total == 0 {
			return 2
		}
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

	// LadybugDB v0.12.2: [:CALLS*1..N] bounded depth (no SHORTEST — parser
	// rejects the SHORTEST keyword in relationship patterns).
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
