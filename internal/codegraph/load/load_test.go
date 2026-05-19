package load_test

import (
	"path/filepath"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/load"
	"github.com/gastownhall/gascity/internal/codegraph/schema"
	"github.com/gastownhall/gascity/internal/codegraph/store"
	"github.com/gastownhall/gascity/internal/codegraph/transform"
)

func TestLoadFull_FromParquet(t *testing.T) {
	tmp := t.TempDir()
	pq := filepath.Join(tmp, "shards")

	// Stage one File + one Function + one DEFINED_IN edge.
	// Omit embedding to avoid FLOAT[512] Parquet type issues (Risk C).
	w := transform.NewWriters(pq)
	w.AddNode(facts.NodeFact{
		Kind:  facts.KindFile,
		Props: map[string]any{"path": "a.go", "lang": "go", "sha": "x", "loc": int32(1)},
	})
	w.AddNode(facts.NodeFact{
		Kind: facts.KindFunction,
		Props: map[string]any{
			"urn": "fn:Add", "name": "Add", "qname": "main.Add",
			"file": "a.go", "start_line": int32(1), "start_col": int32(0),
			"end_line": int32(1), "end_col": int32(20), "signature": "", "doc": "",
			"visibility": "public",
		},
	})
	w.AddEdge(facts.EdgeFact{
		Kind:    facts.EdgeDefinedIn,
		SrcKind: facts.KindFunction, SrcURN: "fn:Add",
		DstKind: facts.KindFile, DstURN: "a.go",
	})
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(tmp, "graph.kuzu"), store.ModeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := load.Full(db, pq, schema.ProfileBase); err != nil {
		t.Fatal(err)
	}

	c := db.Connect()
	defer func() { _ = c.Close() }()
	rows, err := c.Query("MATCH (n:Function)-[:DEFINED_IN]->(f:File) RETURN n.name, f.path;")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.HasNext() {
		t.Fatal("no rows")
	}
	tup, err := rows.Next()
	if err != nil {
		t.Fatal(err)
	}
	defer tup.Close()
	name, _ := tup.GetValue(0)
	path, _ := tup.GetValue(1)
	if name != "Add" || path != "a.go" {
		t.Errorf("got %v %v", name, path)
	}
}
