package scip

import (
	"fmt"
	"strings"

	scippb "github.com/scip-code/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// Parse reads a SCIP protobuf blob and produces node+edge facts. The
// rigName and sha are stamped onto File nodes via the manifest, not onto
// every node (see spec §2 versioning).
func Parse(raw []byte, _ string, sha string) ([]facts.NodeFact, []facts.EdgeFact, error) {
	idx := &scippb.Index{}
	if err := proto.Unmarshal(raw, idx); err != nil {
		return nil, nil, fmt.Errorf("unmarshal scip: %w", err)
	}

	var nodes []facts.NodeFact
	var edges []facts.EdgeFact

	// Build a symbol → SymbolInformation lookup from both document-local
	// symbols and index-level ExternalSymbols.
	symInfo := map[string]*scippb.SymbolInformation{}
	for _, si := range idx.ExternalSymbols {
		symInfo[si.Symbol] = si
	}
	for _, doc := range idx.Documents {
		for _, si := range doc.Symbols {
			symInfo[si.Symbol] = si
		}
	}

	for _, doc := range idx.Documents {
		// File node
		nodes = append(nodes, facts.NodeFact{
			Kind: facts.KindFile,
			URN:  doc.RelativePath,
			Props: map[string]any{
				"path": doc.RelativePath,
				"lang": doc.Language,
				"sha":  sha,
				"loc":  int32(0),
			},
		})

		// One pass: occurrences → defs become nodes, refs become edges.
		for _, occ := range doc.Occurrences {
			if occ.Symbol == "" {
				continue
			}
			info := symInfo[occ.Symbol]

			isDefinition := occ.SymbolRoles&int32(scippb.SymbolRole_Definition) != 0

			if isDefinition {
				if info == nil {
					continue
				}
				kind := mapKind(info.Kind)
				if kind == "" {
					// scip-typescript does not populate SymbolInformation.Kind;
					// fall back to inferring kind from the SCIP descriptor suffix.
					kind = mapKindFromSymbol(occ.Symbol)
				}
				if kind == "" {
					continue
				}
				name := info.DisplayName
				if name == "" {
					name = lastSymbolPart(occ.Symbol)
				}
				nodes = append(nodes, facts.NodeFact{
					Kind: kind,
					URN:  occ.Symbol,
					Props: map[string]any{
						"urn":        occ.Symbol,
						"name":       name,
						"qname":      occ.Symbol,
						"file":       doc.RelativePath,
						"start_line": occ.Range[0],
						"start_col":  occ.Range[1],
						"end_line":   endLine(occ.Range),
						"end_col":    endCol(occ.Range),
						"signature":  info.GetSignatureDocumentation().GetText(),
						"doc":        joinDocs(info.Documentation),
						"visibility": "public",
					},
				})
				edges = append(edges, facts.EdgeFact{
					Kind:    facts.EdgeDefinedIn,
					SrcKind: kind,
					SrcURN:  occ.Symbol,
					DstKind: facts.KindFile,
					DstURN:  doc.RelativePath,
				})
			} else {
				if info == nil {
					continue
				}
				dstKind := mapKind(info.Kind)
				if dstKind == "" {
					dstKind = mapKindFromSymbol(occ.Symbol)
				}
				if dstKind == "" {
					continue
				}
				srcURN := findEnclosing(doc, occ)
				if srcURN == "" {
					continue
				}
				switch dstKind {
				case facts.KindFunction, facts.KindMethod:
					edges = append(edges, facts.EdgeFact{
						Kind:    facts.EdgeCalls,
						SrcKind: facts.KindFunction,
						SrcURN:  srcURN,
						DstKind: dstKind,
						DstURN:  occ.Symbol,
						Props: map[string]any{
							"site_file": doc.RelativePath,
							"site_line": occ.Range[0],
						},
					})
				default:
					edges = append(edges, facts.EdgeFact{
						Kind:    facts.EdgeReferences,
						SrcKind: facts.KindFunction,
						SrcURN:  srcURN,
						DstKind: dstKind,
						DstURN:  occ.Symbol,
						Props:   map[string]any{"kind": "type-ref"},
					})
				}
			}
		}
	}
	return nodes, edges, nil
}

// findEnclosing returns the SCIP symbol URN of the function/method whose
// definition range contains the given occurrence. It first checks
// occ.EnclosingRange (set by scip-go when available), then falls back to
// scanning definition occurrences for the smallest range that contains
// the occurrence line.
func findEnclosing(doc *scippb.Document, occ *scippb.Occurrence) string {
	if len(occ.Range) < 2 {
		return ""
	}
	occLine := occ.Range[0]

	// Fast path: scip-go sets EnclosingRange on many occurrences.
	// Walk the definition occurrences to find the one whose range matches.
	if len(occ.EnclosingRange) >= 3 {
		encSL := occ.EnclosingRange[0]
		encEL := endLine(occ.EnclosingRange)
		var best *scippb.Occurrence
		var bestSize int32 = 1<<30 - 1
		for _, o := range doc.Occurrences {
			if o.SymbolRoles&int32(scippb.SymbolRole_Definition) == 0 {
				continue
			}
			if len(o.Range) < 3 {
				continue
			}
			if o.Range[0] == encSL && endLine(o.Range) == encEL {
				size := endLine(o.Range) - o.Range[0]
				if size < bestSize {
					bestSize = size
					best = o
				}
			}
		}
		if best != nil {
			return best.Symbol
		}
	}

	// Fallback: find the smallest definition range that contains occLine.
	var best *scippb.Occurrence
	var bestSize int32 = 1<<30 - 1
	for _, o := range doc.Occurrences {
		if o.SymbolRoles&int32(scippb.SymbolRole_Definition) == 0 {
			continue
		}
		if len(o.Range) < 3 {
			continue
		}
		sl := o.Range[0]
		el := endLine(o.Range)
		if sl <= occLine && occLine <= el {
			size := el - sl
			if size < bestSize {
				bestSize = size
				best = o
			}
		}
	}
	if best == nil {
		return ""
	}
	return best.Symbol
}

// mapKind converts a SCIP SymbolInformation_Kind to our NodeKind.
// Returns empty string for kinds we don't model.
func mapKind(k scippb.SymbolInformation_Kind) facts.NodeKind {
	switch k {
	case scippb.SymbolInformation_Function:
		return facts.KindFunction
	case scippb.SymbolInformation_Method:
		return facts.KindMethod
	case scippb.SymbolInformation_Class, scippb.SymbolInformation_Struct:
		return facts.KindClass
	case scippb.SymbolInformation_Interface, scippb.SymbolInformation_Trait:
		return facts.KindInterface
	case scippb.SymbolInformation_Field, scippb.SymbolInformation_Property:
		return facts.KindField
	case scippb.SymbolInformation_Module, scippb.SymbolInformation_Namespace, scippb.SymbolInformation_Package:
		return facts.KindModule
	}
	return ""
}

// mapKindFromSymbol infers a NodeKind from the SCIP descriptor suffix of a
// symbol string. This is used as a fallback when SymbolInformation.Kind is
// UnspecifiedKind, which scip-typescript does not populate.
//
// SCIP descriptor suffix conventions:
//   - `name().` or `name()` — function / free-standing method
//   - `TypeName#` — class / type alias / interface
//   - `TypeName#field.` — field / property
//   - `name/` — namespace / module
//
// Local symbols (e.g. "local 3") are skipped by returning "".
func mapKindFromSymbol(sym string) facts.NodeKind {
	if strings.HasPrefix(sym, "local ") {
		return ""
	}
	// Strip trailing dot used on some descriptors to signal "term".
	stripped := strings.TrimSuffix(sym, ".")
	switch {
	case strings.HasSuffix(stripped, ")"):
		// Ends in "()" or "()." — function call descriptor.
		// If it also contains "#" before the "()", it's a method.
		// Split off the last descriptor segment to check.
		if idx := strings.LastIndex(stripped, "#"); idx != -1 && idx > strings.LastIndex(stripped, "/") {
			return facts.KindMethod
		}
		return facts.KindFunction
	case strings.HasSuffix(stripped, "#"):
		// Type descriptor — class / interface / type alias.
		return facts.KindClass
	case strings.Contains(stripped, "#"):
		// Field/property inside a type: "Type#field".
		return facts.KindField
	case strings.HasSuffix(stripped, "/"):
		return facts.KindModule
	}
	return ""
}

func endLine(r []int32) int32 {
	if len(r) >= 3 {
		return r[2]
	}
	if len(r) >= 1 {
		return r[0]
	}
	return 0
}

func endCol(r []int32) int32 {
	if len(r) == 4 {
		return r[3]
	}
	if len(r) >= 2 {
		return r[1]
	}
	return 0
}

// lastSymbolPart returns the final segment of a SCIP symbol string for
// use as a display name when DisplayName is not set.
func lastSymbolPart(sym string) string {
	// SCIP symbol format: "scheme manager package descriptor"
	// Strip trailing punctuation then take the last path/name segment.
	sym = strings.TrimRight(sym, "./()#`")
	for i := len(sym) - 1; i >= 0; i-- {
		switch sym[i] {
		case '/', '.', ' ':
			return sym[i+1:]
		}
	}
	return sym
}

// joinDocs concatenates documentation strings with newlines.
func joinDocs(docs []string) string {
	return strings.Join(docs, "\n")
}
