package scrape

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// TestEncoreCanonicalURN locks the canonical Endpoint URN form to
// "endpoint:<service>.<Method>". If the Encore scraper ever regresses to
// the legacy "<service>:<method>" form, this test fails loudly.
func TestEncoreCanonicalURN(t *testing.T) {
	dir := t.TempDir()
	src := `package auth

import "encore.dev/api"

//encore:api public method=POST path=/auth/login
func Login(ctx context.Context) error { return nil }
`
	pkgDir := filepath.Join(dir, "auth")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "auth.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, edges, err := Encore(dir)
	if err != nil {
		t.Fatal(err)
	}

	var endpointNode *facts.NodeFact
	for i, n := range nodes {
		if n.Kind == facts.KindEndpoint {
			endpointNode = &nodes[i]
			break
		}
	}
	if endpointNode == nil {
		t.Fatal("no Endpoint node emitted")
	}
	want := "endpoint:auth.Login"
	if endpointNode.URN != want {
		t.Errorf("Endpoint URN = %q, want %q (canonical form is endpoint:<service>.<Method>)", endpointNode.URN, want)
	}
	if !strings.HasPrefix(endpointNode.URN, "endpoint:") {
		t.Errorf("Endpoint URN must start with endpoint: prefix; got %q", endpointNode.URN)
	}

	var handles *facts.EdgeFact
	for i, e := range edges {
		if e.Kind == facts.EdgeHandles {
			handles = &edges[i]
			break
		}
	}
	if handles == nil {
		t.Fatal("no HANDLES edge emitted")
	}
	if handles.DstURN != want {
		t.Errorf("HANDLES DstURN = %q, want %q (must match canonical Endpoint URN)", handles.DstURN, want)
	}
}
