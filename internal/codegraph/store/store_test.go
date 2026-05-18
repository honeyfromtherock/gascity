package store_test

import (
	"path/filepath"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/store"
)

func TestOpenWriteReadCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rig.kuzu")

	rw, err := store.Open(path, store.ModeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	conn := rw.Connect()
	if _, err := conn.Exec("CREATE NODE TABLE T(id INT64 PRIMARY KEY, name STRING);"); err != nil {
		t.Fatal(err)
	}
	tx := conn.Begin()
	for i := range int64(100) {
		if _, err := tx.Exec("CREATE (n:T {id:$id, name:$name});",
			map[string]any{"id": i, "name": "row"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := rw.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rw.Close(); err != nil {
		t.Fatal(err)
	}

	ro, err := store.Open(path, store.ModeReadOnly)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close() }()
	c := ro.Connect()
	defer func() { _ = c.Close() }()
	rows, err := c.Query("MATCH (n:T) RETURN count(n);")
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
	got, err := tup.GetValue(0)
	if err != nil {
		t.Fatal(err)
	}
	if got.(int64) != 100 {
		t.Fatalf("got %v want 100", got)
	}
}
