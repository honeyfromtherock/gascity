package scrape

import (
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

func TestReconcileEndpointDstURNs(t *testing.T) {
	canonicalEndpoints := []facts.NodeFact{
		{Kind: facts.KindEndpoint, URN: "endpoint:auth.Login", Props: map[string]any{
			"path":   "/auth/login",
			"method": "POST",
		}},
		{Kind: facts.KindEndpoint, URN: "endpoint:work.GetOrders", Props: map[string]any{
			"path":   "/work-orders/{id}",
			"method": "GET",
		}},
	}
	edges := []facts.EdgeFact{
		{Kind: facts.EdgeCallsEP, SrcKind: facts.KindFile, SrcURN: "fe/Login.tsx",
			DstKind: facts.KindEndpoint, DstURN: "endpoint:path:POST /auth/login",
			Props: map[string]any{"line": 42}},
		{Kind: facts.EdgeCallsEP, SrcKind: facts.KindFile, SrcURN: "ios/Login.swift",
			DstKind: facts.KindEndpoint, DstURN: "endpoint:path:GET /work-orders/{id}",
			Props: map[string]any{"line": 99}},
		{Kind: facts.EdgeCallsEP, SrcKind: facts.KindFile, SrcURN: "ios/Other.swift",
			DstKind: facts.KindEndpoint, DstURN: "endpoint:path:POST /unknown",
			Props: map[string]any{"line": 1}},
	}
	out, matched := ReconcileEndpointDstURNs(edges, canonicalEndpoints)
	if matched != 2 {
		t.Errorf("matched=%d, want 2", matched)
	}
	if out[0].DstURN != "endpoint:auth.Login" {
		t.Errorf("edge[0] DstURN = %q", out[0].DstURN)
	}
	if out[1].DstURN != "endpoint:work.GetOrders" {
		t.Errorf("edge[1] DstURN = %q", out[1].DstURN)
	}
	if out[2].DstURN != "endpoint:path:POST /unknown" {
		t.Errorf("edge[2] should be unchanged, got %q", out[2].DstURN)
	}
	// Rewritten edges record their original URN for debug visibility.
	if got := out[0].Props["original_urn"]; got != "endpoint:path:POST /auth/login" {
		t.Errorf("edge[0] Props[original_urn] = %v, want %q", got, "endpoint:path:POST /auth/login")
	}
	if got := out[1].Props["original_urn"]; got != "endpoint:path:GET /work-orders/{id}" {
		t.Errorf("edge[1] Props[original_urn] = %v", got)
	}
	// Untouched edges do NOT get original_urn (their DstURN was the original).
	if _, ok := out[2].Props["original_urn"]; ok {
		t.Errorf("edge[2] should not have original_urn; was not rewritten")
	}
}
