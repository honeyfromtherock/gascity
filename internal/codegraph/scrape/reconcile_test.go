package scrape_test

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
	"github.com/gastownhall/gascity/internal/codegraph/scrape"
)

func TestReconcileEncoreHandles_MatchesByPkgAndName(t *testing.T) {
	funcs := []facts.NodeFact{
		{Kind: facts.KindFunction, URN: "scip-go gomod encore.app abc `encore.app/backend/auth`/Login()."},
		{Kind: facts.KindFunction, URN: "scip-go gomod encore.app abc `encore.app/backend/auth`/Other()."},
		{Kind: facts.KindFunction, URN: "scip-go gomod encore.app abc `encore.app/backend/billing`/Charge()."},
	}
	handles := []facts.EdgeFact{
		{Kind: facts.EdgeHandles, SrcURN: "scip-go . . auth/Login().", DstURN: "auth:Login"},
		{Kind: facts.EdgeHandles, SrcURN: "scip-go . . billing/Charge().", DstURN: "billing:Charge"},
		{Kind: facts.EdgeHandles, SrcURN: "scip-go . . nonexistent/Missing().", DstURN: "nonexistent:Missing"},
	}
	out := scrape.ReconcileEncoreHandles(handles, funcs)

	if got, want := out[0].SrcURN, "scip-go gomod encore.app abc `encore.app/backend/auth`/Login()."; got != want {
		t.Errorf("Login: got %q, want %q", got, want)
	}
	if got, want := out[1].SrcURN, "scip-go gomod encore.app abc `encore.app/backend/billing`/Charge()."; got != want {
		t.Errorf("Charge: got %q, want %q", got, want)
	}
	// Unmatched stays unchanged (will be IGNORE_ERRORS'd at COPY time)
	if got, want := out[2].SrcURN, "scip-go . . nonexistent/Missing()."; got != want {
		t.Errorf("Missing: got %q, want %q", got, want)
	}
}

func TestReconcileEncoreHandles_EmptyInput(t *testing.T) {
	out := scrape.ReconcileEncoreHandles(nil, nil)
	if len(out) != 0 {
		t.Errorf("expected empty output, got %d edges", len(out))
	}
}

func TestReconcileEncoreHandles_NonFuncNodesIgnored(t *testing.T) {
	funcs := []facts.NodeFact{
		{Kind: facts.KindClass, URN: "scip-go gomod encore.app abc `encore.app/backend/auth`/Login()."},
	}
	handles := []facts.EdgeFact{
		{Kind: facts.EdgeHandles, SrcURN: "scip-go . . auth/Login().", DstURN: "auth:Login"},
	}
	out := scrape.ReconcileEncoreHandles(handles, funcs)
	// Class node should not be used as a match target; SrcURN stays approximate
	if got, want := out[0].SrcURN, "scip-go . . auth/Login()."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReconcileEncoreHandles_MethodNodesMatched(t *testing.T) {
	funcs := []facts.NodeFact{
		{Kind: facts.KindMethod, URN: "scip-go gomod encore.app abc `encore.app/backend/payment`/Handler.Process()."},
	}
	handles := []facts.EdgeFact{
		{Kind: facts.EdgeHandles, SrcURN: "scip-go . . payment/Process().", DstURN: "payment:Process"},
	}
	out := scrape.ReconcileEncoreHandles(handles, funcs)
	if got, want := out[0].SrcURN, "scip-go gomod encore.app abc `encore.app/backend/payment`/Handler.Process()."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReconcileEncoreHandles_PreservesOtherFields(t *testing.T) {
	funcs := []facts.NodeFact{
		{Kind: facts.KindFunction, URN: "scip-go gomod encore.app abc `encore.app/backend/auth`/Login()."},
	}
	handles := []facts.EdgeFact{
		{
			Kind:    facts.EdgeHandles,
			SrcKind: facts.KindFunction,
			SrcURN:  "scip-go . . auth/Login().",
			DstKind: facts.KindEndpoint,
			DstURN:  "auth:Login",
		},
	}
	out := scrape.ReconcileEncoreHandles(handles, funcs)
	if out[0].Kind != facts.EdgeHandles {
		t.Errorf("Kind changed: got %q", out[0].Kind)
	}
	if out[0].DstURN != "auth:Login" {
		t.Errorf("DstURN changed: got %q", out[0].DstURN)
	}
	if out[0].SrcKind != facts.KindFunction {
		t.Errorf("SrcKind changed: got %q", out[0].SrcKind)
	}
}
