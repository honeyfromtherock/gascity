package queries

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// Callers returns the set of callers (up to depth) of the given target URN.
// Picks the first scip-tier rig, falling back to the first registered rig.
// Returns an empty slice on any failure.
func Callers(rigs []rigdir.Rig, urn string, depth, maxResults int) []CallerInfo {
	out := []CallerInfo{}
	if depth <= 0 {
		depth = 1
	}
	if maxResults <= 0 {
		maxResults = 200
	}

	var target rigdir.Rig
	found := false
	for _, r := range rigs {
		if r.Tier == "scip" {
			target = r
			found = true
			break
		}
	}
	if !found && len(rigs) > 0 {
		target = rigs[0]
		found = true
	}
	if !found {
		return out
	}

	path := filepath.Join(target.Root, ".codegraph", "graph.kuzu")
	if _, err := os.Stat(path); err != nil {
		return out
	}
	db, err := store.Open(path, store.ModeReadOnly)
	if err != nil {
		return out
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()

	cypher := fmt.Sprintf(`
			MATCH (caller)-[:CALLS*1..%d]->(target)
			WHERE target.urn = $urn
			RETURN DISTINCT caller.urn AS urn, caller.file AS file, caller.start_line AS line
			LIMIT %d`, depth, maxResults)
	rows, err := conn.Query(cypher, map[string]any{"urn": urn})
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.HasNext() {
		t, err := rows.Next()
		if err != nil {
			break
		}
		urnV, _ := t.GetValue(0)
		fileV, _ := t.GetValue(1)
		lineV, _ := t.GetValue(2)
		t.Close()
		line := 0
		switch v := lineV.(type) {
		case int:
			line = v
		case int32:
			line = int(v)
		case int64:
			line = int(v)
		}
		out = append(out, CallerInfo{
			URN:  fmt.Sprint(urnV),
			File: fmt.Sprint(fileV),
			Line: line,
		})
	}
	return out
}
