package main

import (
	"fmt"
	"os"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

// cmdRigs lists registered rigs and their manifest freshness.
// Each rig's :Manifest node is the source of truth for last-indexed SHA,
// indexed_at timestamp, and indexer version.
func cmdRigs(args []string) int {
	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rigdir: %v\n", err)
		return 1
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rigdir: %v\n", err)
		return 1
	}
	infos := queries.ListRigs(rigs)
	fmt.Printf("%-25s %-9s %-13s %-25s %s\n", "RIG", "TIER", "SHA", "INDEXED_AT", "VERSION")
	for _, info := range infos {
		if info.SHA == "" && info.IndexedAt == "" {
			fmt.Printf("%-25s %-9s %-13s %-25s %s\n", info.Name, info.Tier, "<no graph>", "—", "—")
			continue
		}
		fmt.Printf("%-25s %-9s %-13s %-25s %s\n", info.Name, info.Tier, truncate(info.SHA, 12), info.IndexedAt, info.IndexerVersion)
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
