package scrape

import (
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// ReconcileEdgeSrcURNs rewrites each edge's SrcURN by matching its approximate
// "scip-go . . <pkg>/<func>()." URN against real SCIP Function/Method URNs
// in funcs. Edges whose URN doesn't match the approximate format, or whose
// (pkg, func) tuple isn't in funcs, are passed through unchanged.
//
// This is the generic form used by Encore HANDLES, GoSQL READS_COL/WRITES_COL,
// and GORM READS_COL/WRITES_COL edges — all emit the same approximate format.
//
// Returns the rewritten edges (same length and order as input). Unmatched
// edges are returned unchanged and will be dropped by the loader's
// IGNORE_ERRORS — better than a wrong rewrite.
func ReconcileEdgeSrcURNs(edges []facts.EdgeFact, funcs []facts.NodeFact) []facts.EdgeFact {
	// Build lookup: (pkgName, funcName) → realURN
	realByKey := map[[2]string]string{}
	for _, n := range funcs {
		if n.Kind != facts.KindFunction && n.Kind != facts.KindMethod {
			continue
		}
		pkg, fn := splitSCIPGoURN(n.URN)
		if pkg == "" || fn == "" {
			continue
		}
		realByKey[[2]string{pkg, fn}] = n.URN
	}

	out := make([]facts.EdgeFact, len(edges))
	for i, e := range edges {
		pkg, fn := splitApproxEncoreURN(e.SrcURN)
		if pkg != "" && fn != "" {
			if realURN, ok := realByKey[[2]string{pkg, fn}]; ok {
				e.SrcURN = realURN
			}
		}
		out[i] = e
	}
	return out
}

// ReconcileEncoreHandles is a backward-compatibility alias for ReconcileEdgeSrcURNs.
//
// Deprecated: call ReconcileEdgeSrcURNs directly.
func ReconcileEncoreHandles(handles []facts.EdgeFact, funcs []facts.NodeFact) []facts.EdgeFact {
	return ReconcileEdgeSrcURNs(handles, funcs)
}

// splitApproxEncoreURN parses an approximate Encore-emitted URN of the form
// "scip-go . . auth/Login()." and returns (pkg, funcName).
// The format is fixed by encore.go: "scip-go . . " + service + "/" + funcName + "().".
func splitApproxEncoreURN(urn string) (pkg, fn string) {
	const prefix = "scip-go . . "
	if !strings.HasPrefix(urn, prefix) {
		return "", ""
	}
	rest := strings.TrimPrefix(urn, prefix)
	// rest = "auth/Login()."
	rest = strings.TrimSuffix(rest, ".")
	rest = strings.TrimSuffix(rest, "()")
	// rest = "auth/Login"
	slash := strings.LastIndex(rest, "/")
	if slash < 0 {
		return "", ""
	}
	return rest[:slash], rest[slash+1:]
}

// splitSCIPGoURN parses a real SCIP-go URN of the form
// "scip-go gomod encore.app <ver> `encore.app/backend/auth`/Login()."
// and returns (pkgShortName, funcName).
// The pkg short name is the last path segment inside the backticks.
// The function name is the segment after the closing backtick+slash, before "().".
func splitSCIPGoURN(urn string) (pkg, fn string) {
	// Find backtick-quoted import path
	open := strings.Index(urn, "`")
	if open < 0 {
		return "", ""
	}
	closeIdx := strings.Index(urn[open+1:], "`")
	if closeIdx < 0 {
		return "", ""
	}
	importPath := urn[open+1 : open+1+closeIdx]
	// importPath = "encore.app/backend/auth"
	// pkg short name = last segment
	lastSlash := strings.LastIndex(importPath, "/")
	if lastSlash >= 0 {
		pkg = importPath[lastSlash+1:]
	} else {
		pkg = importPath
	}

	// After the closing backtick: "`/Login()."
	after := urn[open+1+closeIdx+1:] // after the closing backtick
	after = strings.TrimPrefix(after, "/")
	// after = "Login()."
	after = strings.TrimSuffix(after, ".")
	after = strings.TrimSuffix(after, "()")
	// after = "Login"
	// Exclude type receiver prefix "TypeName." if present (method URNs have this)
	if dot := strings.LastIndex(after, "."); dot >= 0 {
		after = after[dot+1:]
	}
	fn = after
	return pkg, fn
}
