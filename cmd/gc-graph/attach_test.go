package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/schema"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// initEmptyGraph creates a graph.kuzu with base schema at dotCodegraph/graph.kuzu.
func initEmptyGraph(t *testing.T, dotCodegraph string) error {
	t.Helper()
	if err := os.MkdirAll(dotCodegraph, 0o755); err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(dotCodegraph, "graph.kuzu"), store.ModeReadWrite)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	return schema.Apply(conn.Inner(), schema.ProfileBase)
}

func TestAttachAllAndQuery(t *testing.T) {
	dir := t.TempDir()
	var rigs []rigdir.Rig
	for _, name := range []string{"a", "b"} {
		root := filepath.Join(dir, name)
		if err := initEmptyGraph(t, filepath.Join(root, ".codegraph")); err != nil {
			t.Fatal(err)
		}
		rigs = append(rigs, rigdir.Rig{Name: name, Root: root, Tier: "scip"})
	}

	hostRoot := filepath.Join(dir, "_host")
	if err := initEmptyGraph(t, filepath.Join(hostRoot, ".codegraph")); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(hostRoot, ".codegraph", "graph.kuzu"), store.ModeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := AttachAll(db, rigs); err != nil {
		t.Fatalf("AttachAll: %v", err)
	}
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	for _, r := range rigs {
		// LadybugDB requires USE <alias> to scope subsequent queries to the
		// attached database; the `alias.NodeLabel` form is not supported.
		if err := conn.Exec("USE " + r.Name + ";"); err != nil {
			t.Errorf("USE %s failed: %v", r.Name, err)
			continue
		}
		res, err := conn.Query("MATCH (f:File) RETURN count(f);")
		if err != nil {
			t.Errorf("query %s failed: %v", r.Name, err)
			continue
		}
		res.Close()
	}
}
