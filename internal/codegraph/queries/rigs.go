package queries

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// ListRigs returns a RigInfo for every registered rig. Rigs without a
// graph.kuzu or without a :Manifest row contribute a RigInfo with only
// Name, Tier, and Profile populated.
func ListRigs(rigs []rigdir.Rig) []RigInfo {
	out := make([]RigInfo, 0, len(rigs))
	for _, r := range rigs {
		info := RigInfo{Name: r.Name, Tier: r.Tier, Profile: r.Profile}
		sha, indexedAt, version, ok := readManifest(r)
		if ok {
			info.SHA = sha
			info.IndexedAt = indexedAt
			info.IndexerVersion = version
		}
		out = append(out, info)
	}
	return out
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
