package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// cmdEndpointConsumers implements `gc graph endpoint-consumers --urn <urn>`.
// Cross-rig query: opens the first rig in ~/.codegraph/rigs.toml as host
// (read-write so ATTACH succeeds), ATTACHes the remaining rigs READ_ONLY,
// then issues a per-rig MATCH for callers of the given endpoint URN.
//
// LadybugDB does not support the `(n:alias.Label)` form, so each attached
// rig is queried in its own statement after `USE <alias>;`.
func cmdEndpointConsumers(args []string) int {
	fs := flag.NewFlagSet("endpoint-consumers", flag.ExitOnError)
	urn := fs.String("urn", "", "Canonical endpoint URN, e.g. endpoint:auth.Login")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *urn == "" {
		fmt.Fprintln(os.Stderr, "usage: gc graph endpoint-consumers --urn <urn>")
		return 2
	}

	rigs, err := LoadRigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(rigs) == 0 {
		fmt.Fprintln(os.Stderr, "no rigs registered in ~/.codegraph/rigs.toml")
		return 1
	}

	fmt.Fprintf(os.Stdout, "%-25s %-9s %-60s %s\n", "RIG", "TIER", "CALLER", "LINE")
	total := 0
	if err := ForEachRig(rigs, func(r rigdir.Rig, alias string, conn *store.Conn) error {
		q := "MATCH (caller)-[c:CALLS_EP]->(e:Endpoint) WHERE e.urn = $urn " +
			"RETURN caller.path, c.site_line LIMIT 200;"
		rows, err := conn.Query(q, map[string]any{"urn": *urn})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] query: %v\n", r.Name, err)
			return nil
		}
		defer rows.Close()
		for rows.HasNext() {
			t, err := rows.Next()
			if err != nil {
				break
			}
			caller, _ := t.GetValue(0)
			line, _ := t.GetValue(1)
			fmt.Fprintf(os.Stdout, "%-25s %-9s %-60s %v\n", r.Name, r.Tier, fmt.Sprint(caller), line)
			t.Close()
			total++
		}
		return nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if total == 0 {
		return 2
	}
	return 0
}
