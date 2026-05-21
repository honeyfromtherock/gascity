package tier2

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

type fakeIdx struct {
	lang  string
	calls []EndpointCall
}

func (f *fakeIdx) Language() string { return f.lang }
func (f *fakeIdx) DetectFiles(root string) ([]string, error) {
	return []string{filepath.Join(root, "Sample.x")}, nil
}
func (f *fakeIdx) ExtractEndpointCalls(path string) ([]EndpointCall, error) {
	out := make([]EndpointCall, len(f.calls))
	for i, c := range f.calls {
		c.FilePath = path
		out[i] = c
	}
	return out, nil
}

func TestRunEmitsFileAndCallsEPFacts(t *testing.T) {
	idx := &fakeIdx{
		lang: "fake",
		calls: []EndpointCall{
			{Line: 10, URL: "/auth/login", Method: "POST"},
			{Line: 20, URL: "/users/{id}", Method: "GET"},
		},
	}
	nodes, edges, err := Run("test-rig", "/tmp/fake-root", []Tier2Indexer{idx})
	if err != nil {
		t.Fatal(err)
	}
	var fileCount int
	for _, n := range nodes {
		if n.Kind == facts.KindFile {
			fileCount++
		}
	}
	if fileCount != 1 {
		t.Errorf("got %d File nodes, want 1", fileCount)
	}
	for _, n := range nodes {
		if n.Kind == facts.KindFile {
			if _, ok := n.Props["rig"]; ok {
				t.Errorf("File node should not have 'rig' prop")
			}
			if _, ok := n.Props["added_at"]; ok {
				t.Errorf("File node should not have 'added_at' prop")
			}
			if n.Props["lang"] == nil {
				t.Errorf("File node missing 'lang' prop")
			}
		}
	}
	if len(edges) != 2 {
		t.Errorf("got %d edges, want 2", len(edges))
	}
	if edges[0].Kind != facts.EdgeCallsEP {
		t.Errorf("edge[0].Kind = %q", edges[0].Kind)
	}
	if !strings.HasPrefix(edges[0].DstURN, "endpoint:path:POST ") {
		t.Errorf("edge[0].DstURN should be approximate, got %q", edges[0].DstURN)
	}
}
