package scrape_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
)

func TestGORM_RecognizesWhereClauseColumns(t *testing.T) {
	_, edges, err := scrape.GORM("testdata/gorm-fixture", "public")
	if err != nil {
		t.Fatal(err)
	}
	wantReads := map[string]bool{
		"public.work_orders.facility_id": false,
		"public.work_orders.created_at":  false,
		"public.facilities.id":           false,
	}
	for _, e := range edges {
		if e.Kind != facts.EdgeReadsCol {
			continue
		}
		dst := e.DstURN
		if _, ok := wantReads[dst]; ok {
			wantReads[dst] = true
		}
	}
	for col, found := range wantReads {
		if !found {
			t.Errorf("expected READS_COL → %s", col)
		}
	}
}
