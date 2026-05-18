package scrape_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
)

func TestAtlasScrape_TablesColumnsFKs(t *testing.T) {
	nodes, edges, err := scrape.Atlas("testdata/atlas-fixture/schema.hcl")
	if err != nil {
		t.Fatal(err)
	}
	tables := map[string]bool{}
	columns := map[string]bool{}
	for _, n := range nodes {
		switch n.Kind {
		case facts.KindDbTable:
			tables[n.URN] = true
		case facts.KindDbColumn:
			columns[n.URN] = true
		}
	}
	for _, want := range []string{"public.work_orders", "public.facilities"} {
		if !tables[want] {
			t.Errorf("missing table %s", want)
		}
	}
	for _, want := range []string{"public.work_orders.facility_id", "public.facilities.id"} {
		if !columns[want] {
			t.Errorf("missing column %s", want)
		}
	}
	fk := false
	for _, e := range edges {
		if e.Kind == facts.EdgeFK &&
			e.SrcURN == "public.work_orders.facility_id" &&
			e.DstURN == "public.facilities.id" {
			fk = true
		}
	}
	if !fk {
		t.Error("FK edge work_orders.facility_id → facilities.id missing")
	}
}
