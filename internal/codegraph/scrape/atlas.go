package scrape

import (
	"fmt"
	"os"

	"ariga.io/atlas/sql/postgres"
	"ariga.io/atlas/sql/schema"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// Atlas reads an Atlas HCL schema file (PostgreSQL dialect) and emits
// DbTable/DbColumn/DbIndex nodes plus FK edges.
func Atlas(hclPath string) ([]facts.NodeFact, []facts.EdgeFact, error) {
	raw, err := os.ReadFile(hclPath)
	if err != nil {
		return nil, nil, err
	}
	var realm schema.Realm
	if err := postgres.EvalHCLBytes(raw, &realm, nil); err != nil {
		return nil, nil, fmt.Errorf("atlas eval: %w", err)
	}
	var nodes []facts.NodeFact
	var edges []facts.EdgeFact
	for _, sch := range realm.Schemas {
		for _, tbl := range sch.Tables {
			tblURN := sch.Name + "." + tbl.Name
			nodes = append(nodes, facts.NodeFact{
				Kind: facts.KindDbTable, URN: tblURN,
				Props: map[string]any{
					"qname": tblURN, "schema_name": sch.Name, "name": tbl.Name,
				},
			})
			for _, col := range tbl.Columns {
				colURN := tblURN + "." + col.Name
				nodes = append(nodes, facts.NodeFact{
					Kind: facts.KindDbColumn, URN: colURN,
					Props: map[string]any{
						"qname": colURN, "table_qname": tblURN,
						"name": col.Name, "type": col.Type.Raw, "nullable": col.Type.Null,
					},
				})
			}
			for _, idx := range tbl.Indexes {
				idxURN := tblURN + "." + idx.Name
				nodes = append(nodes, facts.NodeFact{
					Kind: facts.KindDbIndex, URN: idxURN,
					Props: map[string]any{"qname": idxURN, "table_qname": tblURN, "kind": "index"},
				})
			}
			for _, fk := range tbl.ForeignKeys {
				for i, col := range fk.Columns {
					if i >= len(fk.RefColumns) {
						break
					}
					ref := fk.RefColumns[i]
					srcURN := tblURN + "." + col.Name
					dstURN := fk.RefTable.Schema.Name + "." + fk.RefTable.Name + "." + ref.Name
					edges = append(edges, facts.EdgeFact{
						Kind:    facts.EdgeFK,
						SrcKind: facts.KindDbColumn, SrcURN: srcURN,
						DstKind: facts.KindDbColumn, DstURN: dstURN,
					})
				}
			}
		}
	}
	return nodes, edges, nil
}
