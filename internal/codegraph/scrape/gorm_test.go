package scrape

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

func TestGORM_RecognizesWhereClauseColumns(t *testing.T) {
	_, edges, err := GORM("testdata/gorm-fixture", "public")
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

func TestGormPluralizeSnakeCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"User", "users"},
		{"Status", "statuses"},
		{"Match", "matches"},
		{"Box", "boxes"},
		{"Category", "categories"},
		{"WorkOrder", "work_orders"},
		{"Facility", "facilities"},
		{"Buzz", "buzzes"},
		{"Index", "indexes"},
		{"Branch", "branches"},
	}
	for _, c := range cases {
		if got := gormPluralizeSnakeCase(c.in); got != c.want {
			t.Errorf("gormPluralizeSnakeCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
