package scrape_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
)

func TestEncoreScrape_LoginEndpoint(t *testing.T) {
	nodes, edges, err := scrape.Encore("testdata/encore-fixture")
	if err != nil {
		t.Fatal(err)
	}

	// Expect one Endpoint with route /auth/login
	var ep *facts.NodeFact
	for i := range nodes {
		if nodes[i].Kind == facts.KindEndpoint {
			ep = &nodes[i]
		}
	}
	if ep == nil {
		t.Fatal("no Endpoint node emitted")
	}
	if ep.Props["route"] != "/auth/login" || ep.Props["verb"] != "POST" {
		t.Errorf("got %v want POST /auth/login", ep.Props)
	}
	// Expect HANDLES edge from Login function → endpoint
	found := false
	for _, e := range edges {
		if e.Kind == facts.EdgeHandles && e.DstURN == ep.URN {
			found = true
		}
	}
	if !found {
		t.Error("no HANDLES edge")
	}
}
