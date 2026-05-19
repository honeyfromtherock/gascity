package transform_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/transform"
)

func TestWriters_FlushProducesParquet(t *testing.T) {
	dir := t.TempDir()
	w := transform.NewWriters(dir)
	w.AddNode(facts.NodeFact{
		Kind:  facts.KindFile,
		Props: map[string]any{"path": "a.go", "lang": "go", "sha": "abc", "loc": int32(10)},
	})
	w.AddEdge(facts.EdgeFact{
		Kind:    facts.EdgeDefinedIn,
		SrcKind: facts.KindFunction, SrcURN: "x", DstKind: facts.KindFile, DstURN: "a.go",
	})
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(dir, "nodes", "File.parquet"),
		filepath.Join(dir, "rels", "DEFINED_IN.parquet"),
	} {
		fi, err := os.Stat(p)
		if err != nil || fi.Size() == 0 {
			t.Errorf("expected non-empty %s", p)
		}
	}
}
