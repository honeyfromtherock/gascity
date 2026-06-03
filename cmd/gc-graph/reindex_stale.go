package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// rigStale reports whether the rig's graph is out of date with its git HEAD, or
// absent entirely. A rig with no readable graph / no :Manifest node is stale
// (drives first-index). The authoritative indexed sha is the graph's
// :Manifest.sha — NOT manifest.json on disk, which has been observed to drift.
func rigStale(r rigdir.Rig) bool {
	gitSha := gitSHA(r.Root) // existing helper in reindex.go; "none" if not a git repo
	graphPath := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
	if _, err := os.Stat(graphPath); err != nil {
		return true // no graph yet → stale (first-index)
	}
	manifestSha, ok := readManifestSha(graphPath, r.Name)
	return isStale(gitSha, manifestSha, ok)
}

// isStale is the pure, testable core of the staleness decision.
func isStale(gitSha, manifestSha string, graphHasManifest bool) bool {
	if !graphHasManifest {
		return true
	}
	return gitSha != manifestSha
}

// readManifestSha opens the rig's graph read-only and returns its
// :Manifest.sha. The bool is false if the graph can't be opened or has no
// matching :Manifest row (→ treated as stale). Mirrors the read-only open +
// query pattern in canonicalAgeWarning. A read during a concurrent swap is
// safe: the swap is a single atomic os.Rename of graph.kuzu.
func readManifestSha(graphPath, rigName string) (string, bool) {
	db, err := store.Open(graphPath, store.ModeReadOnly)
	if err != nil {
		return "", false
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	result, err := conn.Query(fmt.Sprintf("MATCH (m:Manifest {rig: '%s'}) RETURN m.sha LIMIT 1;", rigName))
	if err != nil {
		return "", false
	}
	defer result.Close()
	if !result.HasNext() {
		return "", false
	}
	row, err := result.Next()
	if err != nil {
		return "", false
	}
	defer row.Close()
	v, err := row.GetValue(0)
	if err != nil {
		return "", false
	}
	sha, _ := v.(string)
	if sha == "" {
		return "", false
	}
	return sha, true
}
