// Package tier2 is the lightweight indexer for rigs whose languages lack
// usable SCIP indexers (Swift, Kotlin/Android, modern C#). It extracts only
// File nodes and CALLS_EP edges from regex-based per-language extractors.
package tier2

import (
	"fmt"
	"path/filepath"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// EndpointCall is one HTTP-call site discovered in a Tier-2 source file.
type EndpointCall struct {
	FilePath string
	Line     int
	URL      string // raw template, e.g. "/api/work-orders/{id}"
	Method   string // GET, POST, PUT, DELETE, PATCH
	Dynamic  bool   // true when URL contains unresolvable interpolation
}

// Tier2Indexer is the per-language plug-in. Implementations are stateless.
type Tier2Indexer interface {
	Language() string
	DetectFiles(root string) ([]string, error)
	ExtractEndpointCalls(path string) ([]EndpointCall, error)
}

// Run walks the rig with each indexer, collecting File nodes and CALLS_EP edges.
// CALLS_EP edges use approximate DST URNs of the form "endpoint:path:METHOD /path";
// the caller is expected to run scrape.ReconcileEndpointDstURNs against canonical endpoints.
func Run(rig, root string, indexers []Tier2Indexer) ([]facts.NodeFact, []facts.EdgeFact, error) {
	var nodes []facts.NodeFact
	var edges []facts.EdgeFact
	seenFiles := map[string]bool{}
	seenEndpoints := map[string]bool{}

	for _, idx := range indexers {
		paths, err := idx.DetectFiles(root)
		if err != nil {
			return nil, nil, fmt.Errorf("tier2 %s detect: %w", idx.Language(), err)
		}
		for _, p := range paths {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				rel = p
			}
			// URN must match the File table's PK column (path). The loader
			// resolves edge endpoints via the URN string against the PK value.
			urn := rel
			if !seenFiles[urn] {
				nodes = append(nodes, facts.NodeFact{
					Kind: facts.KindFile,
					URN:  urn,
					Props: map[string]any{
						"path": rel,
						"lang": idx.Language(),
					},
				})
				seenFiles[urn] = true
			}
			calls, err := idx.ExtractEndpointCalls(p)
			if err != nil {
				return nil, nil, fmt.Errorf("tier2 %s extract %s: %w", idx.Language(), p, err)
			}
			for _, c := range calls {
				epURN := fmt.Sprintf("endpoint:path:%s %s", c.Method, c.URL)
				// Emit a placeholder :Endpoint node so the CALLS_EP edge has
				// a valid DST in this rig's local graph. Cross-rig queries
				// can reconcile to canonical URNs at query time.
				if !seenEndpoints[epURN] {
					nodes = append(nodes, facts.NodeFact{
						Kind: facts.KindEndpoint,
						URN:  epURN,
						Props: map[string]any{
							"urn":       epURN,
							"transport": "http",
							"route":     c.URL,
							"verb":      c.Method,
						},
					})
					seenEndpoints[epURN] = true
				}
				edges = append(edges, facts.EdgeFact{
					Kind:    facts.EdgeCallsEP,
					SrcKind: facts.KindFile,
					SrcURN:  urn,
					DstKind: facts.KindEndpoint,
					DstURN:  epURN,
					Props: map[string]any{
						"line":    c.Line,
						"dynamic": c.Dynamic,
					},
				})
			}
		}
	}
	return nodes, edges, nil
}
