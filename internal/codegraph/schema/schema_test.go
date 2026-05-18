package schema_test

import (
	"path/filepath"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/schema"
	ladybug "github.com/ladybugdb/go-ladybug"
)

func TestApplySchemaCreatesAllTables(t *testing.T) {
	dir := t.TempDir()
	db, err := ladybug.OpenDatabase(filepath.Join(dir, "s.lbug"), ladybug.DefaultSystemConfig())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	conn, err := ladybug.OpenConnection(db)
	if err != nil {
		t.Fatalf("open conn: %v", err)
	}
	defer conn.Close()

	if err := schema.Apply(conn, schema.ProfileCore); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Idempotence: applying twice must not fail.
	if err := schema.Apply(conn, schema.ProfileCore); err != nil {
		t.Fatalf("apply (idempotent): %v", err)
	}

	wantTables := []string{
		"File", "Module", "Commit",
		"Function", "Method", "Class", "Interface", "Field", "Test", "Endpoint",
		"DbTable", "DbColumn", "DbIndex",
		"CALLS", "REFERENCES", "IMPLEMENTS", "EXTENDS",
		"DEFINED_IN", "METHOD_OF", "DECLARES", "IMPORTS", "TESTS",
		"HANDLES", "CALLS_EP", "MODIFIED_BY", "COCHANGES",
		"FK", "READS_COL", "WRITES_COL",
	}
	res, err := conn.Query("CALL show_tables() RETURN name;")
	if err != nil {
		t.Fatalf("show_tables: %v", err)
	}
	defer res.Close()
	got := map[string]bool{}
	for res.HasNext() {
		tup, err := res.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		v, err := tup.GetValue(0)
		if err != nil {
			t.Fatalf("get value: %v", err)
		}
		name, ok := v.(string)
		if !ok {
			t.Fatalf("expected string table name, got %T", v)
		}
		got[name] = true
	}
	for _, w := range wantTables {
		if !got[w] {
			t.Errorf("missing table %s", w)
		}
	}
}
