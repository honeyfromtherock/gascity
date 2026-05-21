package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// Alias returns a LadybugDB-safe alias for a rig name. Hyphens are replaced
// with underscores because LadybugDB's parser rejects hyphens in identifiers
// used by ATTACH ... AS and USE.
func Alias(rigName string) string {
	return strings.ReplaceAll(rigName, "-", "_")
}

// AttachAll runs ATTACH for each rig's graph.kuzu under the given DB. Each
// rig becomes available under the alias Alias(rig.Name). Attachments are
// read-only — callers performing federated queries should not mutate the
// attached rig databases.
//
// After attachment, callers select an attached database with `USE <alias>;`
// before issuing a MATCH against its node labels. LadybugDB does not
// support an `alias.NodeLabel` form in MATCH patterns.
func AttachAll(db *store.DB, rigs []rigdir.Rig) error {
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	for _, r := range rigs {
		path := filepath.Join(r.Root, ".codegraph", "graph.kuzu")
		q := fmt.Sprintf("ATTACH '%s' AS %s (DBTYPE LBUG, READ_ONLY);", path, Alias(r.Name))
		if err := conn.Exec(q); err != nil {
			return fmt.Errorf("attach %s (%s): %w", r.Name, path, err)
		}
	}
	return nil
}
