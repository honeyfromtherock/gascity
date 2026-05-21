package facts

import "testing"

func TestNodeKindsUnique(t *testing.T) {
	kinds := []NodeKind{
		KindFile, KindModule, KindCommit,
		KindFunction, KindMethod, KindClass, KindInterface,
		KindField, KindTest, KindEndpoint,
		KindDbTable, KindDbColumn, KindDbIndex,
		KindManifest,
	}
	seen := map[NodeKind]bool{}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("empty NodeKind value")
		}
		if seen[k] {
			t.Errorf("duplicate NodeKind value: %q", k)
		}
		seen[k] = true
	}
	if got, want := len(kinds), 14; got != want {
		t.Errorf("kinds list has %d entries; expected %d (update test if you added a new kind)", got, want)
	}
}

func TestEdgeKindsUnique(t *testing.T) {
	kinds := []EdgeKind{
		EdgeCalls, EdgeReferences, EdgeImplements, EdgeExtends,
		EdgeDefinedIn, EdgeMethodOf, EdgeDeclares, EdgeImports, EdgeTests,
		EdgeHandles, EdgeCallsEP, EdgeModifiedBy, EdgeCochanges,
		EdgeFK, EdgeReadsCol, EdgeWritesCol,
	}
	seen := map[EdgeKind]bool{}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("empty EdgeKind value")
		}
		if seen[k] {
			t.Errorf("duplicate EdgeKind value: %q", k)
		}
		seen[k] = true
	}
	if got, want := len(kinds), 16; got != want {
		t.Errorf("kinds list has %d entries; expected %d (update test if you added a new kind)", got, want)
	}
}

func TestFactStructs(t *testing.T) {
	n := NodeFact{Kind: KindFunction, URN: "u", Props: map[string]any{"name": "Foo"}}
	if n.Kind != KindFunction || n.URN != "u" || n.Props["name"] != "Foo" {
		t.Errorf("NodeFact field access broken")
	}
	e := EdgeFact{
		Kind: EdgeCalls, SrcKind: KindFunction, SrcURN: "a",
		DstKind: KindMethod, DstURN: "b", Props: map[string]any{"line": 1},
	}
	if e.Kind != EdgeCalls || e.SrcKind != KindFunction || e.SrcURN != "a" ||
		e.DstKind != KindMethod || e.DstURN != "b" || e.Props["line"] != 1 {
		t.Errorf("EdgeFact field access broken")
	}
}
