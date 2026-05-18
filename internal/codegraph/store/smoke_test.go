package store_test

import (
	"path/filepath"
	"testing"

	lbug "github.com/ladybugdb/go-ladybug"
)

func TestLadybugSmoke(t *testing.T) {
	dir := t.TempDir()
	db, err := lbug.OpenDatabase(filepath.Join(dir, "smoke.lbug"), lbug.DefaultSystemConfig())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	conn, err := lbug.OpenConnection(db)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	res, err := conn.Query("CREATE NODE TABLE Hello(name STRING PRIMARY KEY);")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	res.Close()

	res, err = conn.Query("CREATE (n:Hello {name:'world'});")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	res.Close()

	res, err = conn.Query("MATCH (n:Hello) RETURN n.name;")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer res.Close()

	if !res.HasNext() {
		t.Fatal("no rows")
	}
	tup, err := res.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	defer tup.Close()

	got, err := tup.GetValue(0)
	if err != nil {
		t.Fatalf("get value: %v", err)
	}
	if got != "world" {
		t.Fatalf("got %v, want world", got)
	}
}
