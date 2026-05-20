package scrape_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
)

func TestSQLMigrations_ExtractsTablesColumnsFKs(t *testing.T) {
	nodes, edges, err := scrape.SQLMigrations("testdata/migrations-fixture")
	if err != nil {
		t.Fatal(err)
	}

	tables := map[string]bool{}
	columns := map[string]bool{}
	indexes := map[string]bool{}
	for _, n := range nodes {
		switch n.Kind {
		case facts.KindDbTable:
			tables[n.URN] = true
		case facts.KindDbColumn:
			columns[n.URN] = true
		case facts.KindDbIndex:
			indexes[n.URN] = true
		}
	}
	for _, want := range []string{"public.facilities", "public.work_orders"} {
		if !tables[want] {
			t.Errorf("missing table %s", want)
		}
	}
	for _, want := range []string{
		"public.facilities.id", "public.facilities.name",
		"public.work_orders.id", "public.work_orders.facility_id", "public.work_orders.title",
	} {
		if !columns[want] {
			t.Errorf("missing column %s", want)
		}
	}
	if !indexes["public.work_orders.idx_wo_facility"] {
		t.Error("missing index idx_wo_facility")
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
