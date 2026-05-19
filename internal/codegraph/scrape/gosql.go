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

var (
	selectRE = regexp.MustCompile(`(?is)SELECT\s+(.+?)\s+FROM\s+([a-zA-Z_][\w]*)`)
	insertRE = regexp.MustCompile(`(?is)INSERT\s+INTO\s+([a-zA-Z_][\w]*)\s*\(([^)]+)\)`)
	updateRE = regexp.MustCompile(`(?is)UPDATE\s+([a-zA-Z_][\w]*)\s+SET\s+(.+?)(?:\s+WHERE|$)`)
)

// GoSQL scans Go files under root for sqldb.Query/QueryRow/Exec calls with
// string-literal SQL and emits READS_COL/WRITES_COL edges against the named
// schema. Phase 1 scope: recognizes the encore.dev/storage/sqldb pattern only.
func GoSQL(root, schemaName string) ([]facts.NodeFact, []facts.EdgeFact, error) {
	var edges []facts.EdgeFact

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			srcURN := "scip-go . . " + f.Name.Name + "/" + fn.Name.Name + "()."
			srcKind := facts.KindFunction
			if fn.Recv != nil {
				srcKind = facts.KindMethod
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				method := sel.Sel.Name
				if method != "Query" && method != "QueryRow" && method != "Exec" {
					return true
				}
				if len(call.Args) < 2 {
					return true
				}
				lit, ok := call.Args[1].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				sql := strings.Trim(lit.Value, "`\"")
				edges = append(edges, sqlToEdges(sql, schemaName, srcKind, srcURN)...)
				return true
			})
			return true
		})
		return nil
	})
	return nil, edges, err
}

func sqlToEdges(sql, schemaName string, srcKind facts.NodeKind, srcURN string) []facts.EdgeFact {
	var out []facts.EdgeFact
	if m := selectRE.FindStringSubmatch(sql); m != nil {
		tbl := m[2]
		for _, col := range splitCols(m[1]) {
			out = append(out, facts.EdgeFact{
				Kind: facts.EdgeReadsCol, SrcKind: srcKind, SrcURN: srcURN,
				DstKind: facts.KindDbColumn, DstURN: schemaName + "." + tbl + "." + col,
			})
		}
	}
	if m := insertRE.FindStringSubmatch(sql); m != nil {
		tbl := m[1]
		for _, col := range splitCols(m[2]) {
			out = append(out, facts.EdgeFact{
				Kind: facts.EdgeWritesCol, SrcKind: srcKind, SrcURN: srcURN,
				DstKind: facts.KindDbColumn, DstURN: schemaName + "." + tbl + "." + col,
			})
		}
	}
	if m := updateRE.FindStringSubmatch(sql); m != nil {
		tbl := m[1]
		for _, assign := range strings.Split(m[2], ",") {
			parts := strings.SplitN(strings.TrimSpace(assign), "=", 2)
			col := strings.TrimSpace(parts[0])
			out = append(out, facts.EdgeFact{
				Kind: facts.EdgeWritesCol, SrcKind: srcKind, SrcURN: srcURN,
				DstKind: facts.KindDbColumn, DstURN: schemaName + "." + tbl + "." + col,
			})
		}
	}
	return out
}

func splitCols(list string) []string {
	var out []string
	for _, p := range strings.Split(list, ",") {
		p = strings.TrimSpace(p)
		if i := strings.IndexAny(p, " \t"); i > 0 {
			p = p[:i]
		}
		if i := strings.LastIndex(p, "."); i > 0 {
			p = p[i+1:]
		}
		if p == "*" || p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
