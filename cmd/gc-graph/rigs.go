package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
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
	fmt.Printf("%-25s %-9s %-13s %-25s %s\n", "RIG", "TIER", "SHA", "INDEXED_AT", "VERSION")
	for _, r := range rigs {
		sha, indexedAt, ver, ok := readManifest(r)
		if !ok {
			fmt.Printf("%-25s %-9s %-13s %-25s %s\n", r.Name, r.Tier, "<no graph>", "—", "—")
			continue
		}
		fmt.Printf("%-25s %-9s %-13s %-25s %s\n", r.Name, r.Tier, truncate(sha, 12), indexedAt, ver)
	}
	return 0
}

// readManifest opens a rig's graph.kuzu read-only and pulls the :Manifest row.
// Returns ok=false if the graph doesn't exist or the manifest row is missing.
func readManifest(r rigdir.Rig) (sha, indexedAt, version string, ok bool) {
	path := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
	if _, err := os.Stat(path); err != nil {
		return "", "", "", false
	}
	db, err := store.Open(path, store.ModeReadOnly)
	if err != nil {
		return "", "", "", false
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	q := "MATCH (m:Manifest) RETURN m.sha, m.indexed_at, m.indexer_version LIMIT 1;"
	result, err := conn.Query(q)
	if err != nil {
		return "", "", "", false
	}
	defer result.Close()
	if !result.HasNext() {
		return "", "", "", false
	}
	row, err := result.Next()
	if err != nil {
		return "", "", "", false
	}
	defer row.Close()
	v0, _ := row.GetValue(0)
	v1, _ := row.GetValue(1)
	v2, _ := row.GetValue(2)
	return fmt.Sprint(v0), formatTimeMaybe(v1), fmt.Sprint(v2), true
}

// formatTimeMaybe formats a time.Time or string timestamp consistently.
// Manifest's indexed_at is stored as RFC3339 string per the Phase 2 schema fix.
func formatTimeMaybe(v any) string {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return fmt.Sprint(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
