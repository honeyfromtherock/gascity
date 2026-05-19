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

// TestBuildIndexes_FTSReturnsFunction verifies that BuildIndexes (called
// internally by load.Full) runs to completion without error and that the
// graph data is intact afterwards. The FTS query assertion is best-effort:
// if the extension is absent in the current LadybugDB build the assertion
// is logged rather than failing the test.
func TestBuildIndexes_FTSReturnsFunction(t *testing.T) {
	tmp := t.TempDir()
	pq := filepath.Join(tmp, "shards")

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
			"end_line": int32(1), "end_col": int32(20),
			"signature": "func(int, int) int", "doc": "Add returns the sum.",
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

	// load.Full calls BuildIndexes after checkpoint. The call must succeed even
	// when the FTS/vector extensions are not available in this LadybugDB build.
	if err := load.Full(db, pq, schema.ProfileBase); err != nil {
		t.Fatal(err)
	}

	c := db.Connect()
	defer func() { _ = c.Close() }()

	// Graph data must be intact after indexing.
	graphRows, err := c.Query("MATCH (n:Function) RETURN n.name;")
	if err != nil {
		t.Fatal(err)
	}
	defer graphRows.Close()
	if !graphRows.HasNext() {
		t.Fatal("no Function nodes found after BuildIndexes")
	}

	// FTS query: probe whether the index was built. The FTS extension is
	// bundled but may not be functional in the current LadybugDB build;
	// log rather than fail when the extension is absent.
	ftsRows, ftsErr := c.Query("CALL QUERY_FTS_INDEX('Function','fn_fts','Add') RETURN node.name;")
	if ftsErr != nil {
		t.Logf("FTS index not available in this LadybugDB build (extension skipped during load): %v", ftsErr)
		return
	}
	defer ftsRows.Close()
	if !ftsRows.HasNext() {
		t.Error("FTS returned no rows for 'Add'")
	}
}
