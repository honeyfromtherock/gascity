package scrape_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
)

func TestGoSQL_ReadsAndWritesColumns(t *testing.T) {
	_, edges, err := scrape.GoSQL("testdata/gosql-fixture", "public")
	if err != nil {
		t.Fatal(err)
	}
	reads, writes := false, false
	for _, e := range edges {
		switch e.Kind {
		case facts.EdgeReadsCol:
			if e.DstURN == "public.work_orders.title" {
				reads = true
			}
		case facts.EdgeWritesCol:
			if e.DstURN == "public.work_orders.title" || e.DstURN == "public.work_orders.id" {
				writes = true
			}
		}
	}
	if !reads {
		t.Error("expected READS_COL → public.work_orders.title")
	}
	if !writes {
		t.Error("expected WRITES_COL → public.work_orders.title or .id")
	}
}
