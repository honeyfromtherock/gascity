// Package scrape extracts non-SCIP facts (contracts, schemas) from a rig.
package scrape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

var annotationRE = regexp.MustCompile(`//encore:api\s+(\w+)?\s*(?:method=(\w+))?\s*(?:path=(\S+))?`)

// Encore scans Go files under root for //encore:api annotations and emits
// Endpoint + HANDLES facts for each. The HANDLES edge's SrcURN is an
// approximate SCIP-style URN; the loader's reconciliation pass cross-
// references it against the real SCIP-produced URN at apply time.
func Encore(root string) ([]facts.NodeFact, []facts.EdgeFact, error) {
	var nodes []facts.NodeFact
	var edges []facts.EdgeFact

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		return scanFile(path, root, &nodes, &edges)
	})
	return nodes, edges, err
}

func scanFile(path, root string, nodes *[]facts.NodeFact, edges *[]facts.EdgeFact) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil // skip unparseable files; SCIP indexer will surface the real error
	}
	rel, _ := filepath.Rel(root, path)
	_ = rel
	pkgName := f.Name.Name

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}
		for _, c := range fn.Doc.List {
			m := annotationRE.FindStringSubmatch(c.Text)
			if m == nil {
				continue
			}
			verb, route := m[2], m[3]
			if verb == "" {
				verb = "POST"
			}
			service := pkgName
			urn := "endpoint:" + service + "." + fn.Name.Name
			funcURN := "scip-go . . " + service + "/" + fn.Name.Name + "()."

			*nodes = append(*nodes, facts.NodeFact{
				Kind: facts.KindEndpoint,
				URN:  urn,
				Props: map[string]any{
					"urn": urn, "transport": "encore", "route": route, "verb": verb,
				},
			})
			*edges = append(*edges, facts.EdgeFact{
				Kind:    facts.EdgeHandles,
				SrcKind: facts.KindFunction,
				SrcURN:  funcURN,
				DstKind: facts.KindEndpoint,
				DstURN:  urn,
			})
		}
	}
	return nil
}
