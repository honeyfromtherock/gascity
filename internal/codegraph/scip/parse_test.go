package scip_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/scip"
)

func TestParseSCIP_TinyGo_NodesAndEdges(t *testing.T) {
	if _, err := exec.LookPath("scip-go"); err != nil {
		t.Skip("scip-go not installed")
	}
	tmp := t.TempDir()
	out := filepath.Join(tmp, "index.scip")
	if err := (scip.GoRunner{}).Run("testdata/tinygo", out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, err := scip.Parse(raw, "tinygo-rig", "abc1234")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("nodes=%d edges=%d", len(nodes), len(edges))
	for _, n := range nodes {
		t.Logf("  node kind=%s urn=%s name=%v", n.Kind, n.URN, n.Props["name"])
	}
	for _, e := range edges {
		t.Logf("  edge kind=%s src=%s dst=%s", e.Kind, e.SrcURN, e.DstURN)
	}

	// Sanity: Add function appears once.
	addFound := false
	for _, n := range nodes {
		if n.Kind == "Function" && n.Props["name"] == "Add" {
			addFound = true
			break
		}
	}
	if !addFound {
		t.Errorf("Add function not found in %d nodes", len(nodes))
	}

	// main calls Add → at least one CALLS edge
	callFound := false
	for _, e := range edges {
		if e.Kind == "CALLS" {
			callFound = true
			break
		}
	}
	if !callFound {
		t.Errorf("no CALLS edge in %d edges", len(edges))
	}

	// External symbol placeholder: fmt.Println should appear as a Function node
	// even though it's defined in the stdlib (not indexed here).
	externalFound := false
	for _, n := range nodes {
		if n.Kind == "Function" && strings.Contains(n.Props["name"].(string), "Println") {
			externalFound = true
			if v := n.Props["visibility"]; v != "external" {
				t.Errorf("Println visibility = %v, want external", v)
			}
			break
		}
	}
	if !externalFound {
		t.Error("fmt.Println placeholder node missing")
	}
}
