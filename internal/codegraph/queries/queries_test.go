package queries

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/schema"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// initEmptyGraph creates an empty graph.kuzu under root with the base schema
// applied, and returns the corresponding rigdir.Rig descriptor.
func initEmptyGraph(t *testing.T, root, name, tier string) rigdir.Rig {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".codegraph"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(root, ".codegraph", "graph.kuzu")
	db, err := store.Open(dbPath, store.ModeReadWrite)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	conn := db.Connect()
	if err := schema.Apply(conn.Inner(), schema.ProfileBase); err != nil {
		_ = conn.Close()
		_ = db.Close()
		t.Fatalf("schema: %v", err)
	}
	_ = conn.Close()
	_ = db.Close()
	return rigdir.Rig{Name: name, Root: root, Tier: tier}
}

func TestListRigsEmptyGraph(t *testing.T) {
	root := t.TempDir()
	r := initEmptyGraph(t, root, "rig-a", "scip")
	got := ListRigs([]rigdir.Rig{r})
	if len(got) != 1 {
		t.Fatalf("want 1 rig, got %d", len(got))
	}
	if got[0].Name != "rig-a" || got[0].Tier != "scip" {
		t.Errorf("got %+v", got[0])
	}
	// Empty graph has no :Manifest row; manifest fields should be empty.
	if got[0].SHA != "" || got[0].IndexedAt != "" {
		t.Errorf("expected empty manifest fields, got SHA=%q IndexedAt=%q", got[0].SHA, got[0].IndexedAt)
	}
}

func TestListEndpointsEmptyGraph(t *testing.T) {
	root := t.TempDir()
	r := initEmptyGraph(t, root, "rig-a", "endpoint")
	got := ListEndpoints([]rigdir.Rig{r}, "", "", 50)
	if len(got) != 0 {
		t.Errorf("want 0 endpoints, got %d", len(got))
	}
}

func TestEndpointConsumersEmptyGraph(t *testing.T) {
	root := t.TempDir()
	r := initEmptyGraph(t, root, "rig-a", "scip")
	got := EndpointConsumers([]rigdir.Rig{r}, "endpoint:nothing.Here")
	if len(got) != 0 {
		t.Errorf("want 0 consumers, got %d", len(got))
	}
}

func TestBlastEmptyGraph(t *testing.T) {
	root := t.TempDir()
	r := initEmptyGraph(t, root, "rig-a", "scip")
	got := Blast([]rigdir.Rig{r}, "", "endpoint:foo.Bar", 3, 100)
	if got.Symbols == nil || got.Files == nil || got.Tests == nil || got.Endpoints == nil || got.DbColumns == nil {
		t.Errorf("nil slice in BlastResult: %+v", got)
	}
	if len(got.Symbols) != 0 || len(got.Tests) != 0 || len(got.Endpoints) != 0 || len(got.DbColumns) != 0 {
		t.Errorf("expected empty results, got %+v", got)
	}
}

func TestCallersEmptyGraph(t *testing.T) {
	root := t.TempDir()
	r := initEmptyGraph(t, root, "rig-a", "scip")
	got := Callers([]rigdir.Rig{r}, "endpoint:foo.Bar", 2, 50)
	if len(got) != 0 {
		t.Errorf("want 0 callers, got %d", len(got))
	}
}
