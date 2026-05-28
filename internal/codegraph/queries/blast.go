package queries

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// Blast returns the structured blast-radius bundle for either a file path or
// a URN. If file is provided and urn is empty, the file is resolved to a
// (rig, rel-path) by prefix-matching the registered rigs; rel becomes the
// subject. Otherwise urn is used directly and the first scip-tier rig (or
// the first rig if none is scip) is queried.
//
// On any open/query failure, Blast returns a zero-valued BlastResult with
// non-nil empty slices.
func Blast(rigs []rigdir.Rig, file, urn string, depth, maxResults int) BlastResult {
	result := BlastResult{
		Symbols:   []string{},
		Files:     []string{},
		Tests:     []string{},
		Endpoints: []string{},
		DbColumns: []string{},
	}
	if depth <= 0 {
		depth = 3
	}
	if maxResults <= 0 {
		maxResults = 500
	}

	subject := urn
	var target rigdir.Rig
	var found bool

	if file != "" && urn == "" {
		abs, err := filepath.Abs(file)
		if err != nil {
			return result
		}
		for _, r := range rigs {
			rootAbs, _ := filepath.Abs(r.Root)
			if !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) && abs != rootAbs {
				continue
			}
			rel, _ := filepath.Rel(rootAbs, abs)
			subject = rel
			target = r
			found = true
			break
		}
		if !found {
			return result
		}
	} else {
		// Prefer the first scip-tier rig; fall back to first rig.
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
			return result
		}
	}

	result.Subject = subject

	path := filepath.Join(target.Root, ".codegraph", "graph.kuzu")
	if _, err := os.Stat(path); err != nil {
		return result
	}
	db, err := store.Open(path, store.ModeReadOnly)
	if err != nil {
		return result
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()

	queries := map[string]string{
		"symbols": fmt.Sprintf(
			`MATCH (n)-[:CALLS|REFERENCES*1..%d]->(t) WHERE t.urn=$urn
				 RETURN DISTINCT n.qname LIMIT %d`, depth, maxResults),
		"tests": fmt.Sprintf(`MATCH (t:Test)-[:TESTS|CALLS*1..4]->(s) WHERE s.urn=$urn
				   RETURN DISTINCT t.file LIMIT %d`, maxResults),
		"endpoints": fmt.Sprintf(`MATCH (n)-[:HANDLES|CALLS_EP]->(e:Endpoint)
		               WHERE n.urn=$urn OR n.path=$urn
					   RETURN DISTINCT e.urn LIMIT %d`, maxResults),
		"db_columns": fmt.Sprintf(`MATCH (n)-[:READS_COL|WRITES_COL]->(c:DbColumn) WHERE n.urn=$urn
						RETURN DISTINCT c.qname LIMIT %d`, maxResults),
	}

	for label, cypher := range queries {
		rows, err := conn.Query(cypher, map[string]any{"urn": subject})
		if err != nil {
			continue
		}
		for rows.HasNext() {
			t, err := rows.Next()
			if err != nil {
				break
			}
			v, _ := t.GetValue(0)
			t.Close()
			str := fmt.Sprint(v)
			switch label {
			case "symbols":
				result.Symbols = append(result.Symbols, str)
			case "tests":
				result.Tests = append(result.Tests, str)
			case "endpoints":
				result.Endpoints = append(result.Endpoints, str)
			case "db_columns":
				result.DbColumns = append(result.DbColumns, str)
			}
		}
		rows.Close()
	}
	return result
}
