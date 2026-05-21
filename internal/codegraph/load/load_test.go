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

func fnNode(urn, name, file string) facts.NodeFact {
	return facts.NodeFact{
		Kind: facts.KindFunction,
		Props: map[string]any{
			"urn": urn, "name": name, "qname": name,
			"file": file, "start_line": int32(1), "start_col": int32(0),
			"end_line": int32(5), "end_col": int32(1),
			"signature": "", "doc": "", "visibility": "public",
		},
	}
}

func methodNode(urn, name, file string) facts.NodeFact {
	return facts.NodeFact{
		Kind: facts.KindMethod,
		Props: map[string]any{
			"urn": urn, "name": name, "qname": name,
			"file": file, "start_line": int32(1), "start_col": int32(0),
			"end_line": int32(5), "end_col": int32(1),
			"signature": "", "doc": "", "receiver": "T", "visibility": "public",
		},
	}
}

func fileNode(path string) facts.NodeFact {
	return facts.NodeFact{
		Kind:  facts.KindFile,
		Props: map[string]any{"path": path, "lang": "go", "sha": "x", "loc": int32(10)},
	}
}

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

func TestLoadFull_MultiPairRel(t *testing.T) {
	tmp := t.TempDir()
	pq := filepath.Join(tmp, "shards")

	// Stage 2 Functions + 1 Method + 2 CALLS edges:
	// Function→Function and Method→Method as separate src/dst kind pairs.
	w := transform.NewWriters(pq)
	w.AddNode(fileNode("b.go"))
	w.AddNode(fnNode("fn:Caller", "Caller", "b.go"))
	w.AddNode(fnNode("fn:Callee", "Callee", "b.go"))
	w.AddNode(methodNode("mth:DoWork", "DoWork", "b.go"))
	w.AddNode(methodNode("mth:Helper", "Helper", "b.go"))

	// CALLS Function→Function
	w.AddEdge(facts.EdgeFact{
		Kind:    facts.EdgeCalls,
		SrcKind: facts.KindFunction, SrcURN: "fn:Caller",
		DstKind: facts.KindFunction, DstURN: "fn:Callee",
		Props: map[string]any{"site_file": "b.go", "site_line": int32(3)},
	})
	// CALLS Method→Method
	w.AddEdge(facts.EdgeFact{
		Kind:    facts.EdgeCalls,
		SrcKind: facts.KindMethod, SrcURN: "mth:DoWork",
		DstKind: facts.KindMethod, DstURN: "mth:Helper",
		Props: map[string]any{"site_file": "b.go", "site_line": int32(10)},
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
	rows, err := c.Query("MATCH ()-[r:CALLS]->() RETURN COUNT(r);")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.HasNext() {
		t.Fatal("no count row")
	}
	tup, err := rows.Next()
	if err != nil {
		t.Fatal(err)
	}
	defer tup.Close()
	count, _ := tup.GetValue(0)
	if count != int64(2) {
		t.Errorf("CALLS count = %v, want 2", count)
	}
}

func TestLoadFull_IgnoresMissingFK(t *testing.T) {
	tmp := t.TempDir()
	pq := filepath.Join(tmp, "shards")

	// Stage 1 Function "fn:A" and a CALLS edge targeting "fn:B" (doesn't exist).
	w := transform.NewWriters(pq)
	w.AddNode(fileNode("c.go"))
	w.AddNode(fnNode("fn:A", "A", "c.go"))
	w.AddEdge(facts.EdgeFact{
		Kind:    facts.EdgeCalls,
		SrcKind: facts.KindFunction, SrcURN: "fn:A",
		DstKind: facts.KindFunction, DstURN: "fn:B-nonexistent",
		Props: map[string]any{"site_file": "c.go", "site_line": int32(2)},
	})
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(tmp, "graph.kuzu"), store.ModeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	// Must not return an error even though the FK target is missing.
	if err := load.Full(db, pq, schema.ProfileBase); err != nil {
		t.Fatalf("load.Full errored on missing FK: %v", err)
	}

	c := db.Connect()
	defer func() { _ = c.Close() }()
	rows, err := c.Query("MATCH ()-[r:CALLS]->() RETURN COUNT(r);")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.HasNext() {
		t.Fatal("no count row")
	}
	tup, err := rows.Next()
	if err != nil {
		t.Fatal(err)
	}
	defer tup.Close()
	count, _ := tup.GetValue(0)
	if count != int64(0) {
		t.Errorf("CALLS count = %v, want 0 (dangling edge should be silently dropped)", count)
	}
}

func TestLoadFull_ProfileCoreDbTables(t *testing.T) {
	tmp := t.TempDir()
	pq := filepath.Join(tmp, "shards")

	// Stage 1 DbTable + 2 DbColumn nodes + 1 FK edge between the columns.
	w := transform.NewWriters(pq)
	w.AddNode(facts.NodeFact{
		Kind: facts.KindDbTable,
		Props: map[string]any{
			"qname": "public.orders", "schema_name": "public", "name": "orders",
		},
	})
	w.AddNode(facts.NodeFact{
		Kind: facts.KindDbColumn,
		Props: map[string]any{
			"qname": "public.orders.id", "table_qname": "public.orders",
			"name": "id", "type": "uuid", "nullable": false,
		},
	})
	w.AddNode(facts.NodeFact{
		Kind: facts.KindDbColumn,
		Props: map[string]any{
			"qname": "public.orders.customer_id", "table_qname": "public.orders",
			"name": "customer_id", "type": "uuid", "nullable": true,
		},
	})
	w.AddEdge(facts.EdgeFact{
		Kind:    facts.EdgeFK,
		SrcKind: facts.KindDbColumn, SrcURN: "public.orders.customer_id",
		DstKind: facts.KindDbColumn, DstURN: "public.orders.id",
	})
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(tmp, "graph.kuzu"), store.ModeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := load.Full(db, pq, schema.ProfileCore); err != nil {
		t.Fatal(err)
	}

	c := db.Connect()
	defer func() { _ = c.Close() }()

	for _, tc := range []struct {
		query string
		want  int64
		label string
	}{
		{"MATCH (t:DbTable) RETURN COUNT(t);", 1, "DbTable"},
		{"MATCH (c:DbColumn) RETURN COUNT(c);", 2, "DbColumn"},
		{"MATCH ()-[r:FK]->() RETURN COUNT(r);", 1, "FK"},
	} {
		rows, err := c.Query(tc.query)
		if err != nil {
			t.Fatalf("%s query: %v", tc.label, err)
		}
		if !rows.HasNext() {
			rows.Close()
			t.Fatalf("%s: no count row", tc.label)
		}
		tup, err := rows.Next()
		if err != nil {
			rows.Close()
			t.Fatalf("%s: %v", tc.label, err)
		}
		count, _ := tup.GetValue(0)
		tup.Close()
		rows.Close()
		if count != tc.want {
			t.Errorf("%s count = %v, want %d", tc.label, count, tc.want)
		}
	}
}
