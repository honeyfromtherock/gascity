// Package scrape provides scrapers for extracting code graph facts from
// source files and schema definitions.
package scrape

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// SQLMigrations walks a directory of timestamp-sorted .sql migration files
// and extracts the cumulative schema (tables, columns, indexes, FKs) by
// parsing CREATE TABLE, CREATE INDEX, and ALTER TABLE ADD CONSTRAINT
// statements. Designed as a fallback for repos whose Atlas HCL uses Pro
// features (external_schema, composite_schema) that scrape.Atlas can't
// parse — the migrations are the authoritative DDL anyway.
//
// Scope: best-effort regex-based extraction. CREATE TABLE column lists are
// the only complex parse; everything else is line-oriented. ALTER TABLE
// DROP and RENAME are NOT applied — Phase 1 accepts the resulting schema
// as approximate.
func SQLMigrations(dir string) ([]facts.NodeFact, []facts.EdgeFact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	files := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files) // timestamp prefix → correct application order

	state := newSchemaState()
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue // skip unreadable
		}
		applyMigration(state, string(raw))
	}
	return state.facts()
}

type schemaState struct {
	tables  map[string]*tableInfo // qname → info
	indexes map[string]string     // index_qname → table_qname
	fks     []fkInfo
}

type tableInfo struct {
	schema, name string
	columns      map[string]*colInfo // col_name → info
	colOrder     []string
}

type colInfo struct {
	name, typ string
	nullable  bool
}

type fkInfo struct {
	srcTable, srcCol, dstTable, dstCol string
}

func newSchemaState() *schemaState {
	return &schemaState{
		tables:  map[string]*tableInfo{},
		indexes: map[string]string{},
	}
}

var (
	// createTableRE matches: CREATE TABLE "schema"."table" ( body )
	createTableRE = regexp.MustCompile(`(?is)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?"([a-zA-Z_]\w*)"\."([a-zA-Z_]\w*)"\s*\((.+?)\)\s*;`)
	// createTableNoSchemaRE matches: CREATE TABLE "table" ( body ) — no schema qualifier
	createTableNoSchemaRE = regexp.MustCompile(`(?is)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?"([a-zA-Z_]\w*)"\s*\((.+?)\)\s*;`)
	// createIndexRE matches: CREATE [UNIQUE] INDEX "name" ON "schema"."table" (
	createIndexRE = regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF NOT EXISTS\s+)?"?([a-zA-Z_]\w*)"?\s+ON\s+"?([a-zA-Z_]\w*)"?\."?([a-zA-Z_]\w*)"?\s*\(`)
	// createIndexNoSchemaRE matches: CREATE [UNIQUE] INDEX "name" ON "table" (
	createIndexNoSchemaRE = regexp.MustCompile(`(?is)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF NOT EXISTS\s+)?"?([a-zA-Z_]\w*)"?\s+ON\s+"?([a-zA-Z_]\w*)"?\s*\(`)
	// alterAddFKRE matches ALTER TABLE "schema"."table" ADD CONSTRAINT "name" FOREIGN KEY ("col") REFERENCES "schema"."table" ("col")
	alterAddFKRE = regexp.MustCompile(`(?is)ALTER TABLE\s+"([a-zA-Z_]\w*)"\."([a-zA-Z_]\w*)"\s+ADD CONSTRAINT\s+"?[a-zA-Z_]\w*"?\s+FOREIGN KEY\s*\(\s*"?([a-zA-Z_]\w*)"?\s*\)\s+REFERENCES\s+"([a-zA-Z_]\w*)"\."([a-zA-Z_]\w*)"\s*\(\s*"?([a-zA-Z_]\w*)"?\s*\)`)
	// alterAddFKNoSchemaRE matches the no-schema variant for both src and dst
	alterAddFKNoSchemaRE = regexp.MustCompile(`(?is)ALTER TABLE\s+"?([a-zA-Z_]\w*)"?\s+ADD CONSTRAINT\s+"?[a-zA-Z_]\w*"?\s+FOREIGN KEY\s*\(\s*"?([a-zA-Z_]\w*)"?\s*\)\s+REFERENCES\s+"?([a-zA-Z_]\w*)"?\s*\(\s*"?([a-zA-Z_]\w*)"?\s*\)`)
	// columnLineRE extracts column name and type from a single column definition line
	columnLineRE = regexp.MustCompile(`(?i)^\s*"?([a-zA-Z_]\w*)"?\s+([a-zA-Z_][\w\s\(\),]*)`)
)

func applyMigration(s *schemaState, sql string) {
	// CREATE TABLE with explicit schema
	for _, m := range createTableRE.FindAllStringSubmatch(sql, -1) {
		s.applyCreateTable(m[1], m[2], m[3])
	}
	// CREATE TABLE without schema (default to public) — only if not already captured above
	for _, m := range createTableNoSchemaRE.FindAllStringSubmatch(sql, -1) {
		qname := "public." + m[1]
		if _, ok := s.tables[qname]; !ok {
			s.applyCreateTable("public", m[1], m[2])
		}
	}
	// CREATE INDEX with explicit schema
	for _, m := range createIndexRE.FindAllStringSubmatch(sql, -1) {
		idxName := m[1]
		tblQname := m[2] + "." + m[3]
		s.indexes[tblQname+"."+idxName] = tblQname
	}
	// CREATE INDEX without schema
	for _, m := range createIndexNoSchemaRE.FindAllStringSubmatch(sql, -1) {
		idxName := m[1]
		tblQname := "public." + m[2]
		idxQname := tblQname + "." + idxName
		if _, ok := s.indexes[idxQname]; !ok {
			s.indexes[idxQname] = tblQname
		}
	}
	// ALTER TABLE ADD FOREIGN KEY with explicit schemas on both sides
	for _, m := range alterAddFKRE.FindAllStringSubmatch(sql, -1) {
		s.fks = append(s.fks, fkInfo{
			srcTable: m[1] + "." + m[2], srcCol: m[3],
			dstTable: m[4] + "." + m[5], dstCol: m[6],
		})
	}
	// ALTER TABLE ADD FOREIGN KEY no-schema variant
	for _, m := range alterAddFKNoSchemaRE.FindAllStringSubmatch(sql, -1) {
		fk := fkInfo{
			srcTable: "public." + m[1], srcCol: m[2],
			dstTable: "public." + m[3], dstCol: m[4],
		}
		// Avoid duplicates already captured by the schema-aware regex
		dupe := false
		for _, existing := range s.fks {
			if existing == fk {
				dupe = true
				break
			}
		}
		if !dupe {
			s.fks = append(s.fks, fk)
		}
	}
}

func (s *schemaState) applyCreateTable(schemaName, tableName, body string) {
	qname := schemaName + "." + tableName
	tbl := &tableInfo{
		schema:  schemaName,
		name:    tableName,
		columns: map[string]*colInfo{},
	}
	// Split body by comma at paren-depth 0 to get individual column/constraint defs
	depth := 0
	start := 0
	parts := []string{}
	for i, ch := range body {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, body[start:])

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		upper := strings.ToUpper(p)
		// Skip constraint-only lines
		if strings.HasPrefix(upper, "PRIMARY KEY") ||
			strings.HasPrefix(upper, "FOREIGN KEY") ||
			strings.HasPrefix(upper, "CONSTRAINT") ||
			strings.HasPrefix(upper, "UNIQUE") ||
			strings.HasPrefix(upper, "CHECK") {
			continue
		}
		m := columnLineRE.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		colName := m[1]
		colType := strings.TrimSpace(m[2])
		// Strip trailing noise from colType (NOT NULL, DEFAULT, etc.)
		for _, stop := range []string{" NOT NULL", " NULL", " DEFAULT", " REFERENCES", " CHECK", " PRIMARY", " UNIQUE"} {
			if idx := strings.Index(strings.ToUpper(colType), stop); idx >= 0 {
				colType = strings.TrimSpace(colType[:idx])
			}
		}
		nullable := !strings.Contains(upper, "NOT NULL")
		if _, exists := tbl.columns[colName]; !exists {
			tbl.colOrder = append(tbl.colOrder, colName)
		}
		tbl.columns[colName] = &colInfo{name: colName, typ: colType, nullable: nullable}
	}
	s.tables[qname] = tbl
}

func (s *schemaState) facts() ([]facts.NodeFact, []facts.EdgeFact, error) {
	var nodes []facts.NodeFact
	var edges []facts.EdgeFact

	// Sort table names for stable output
	tableNames := make([]string, 0, len(s.tables))
	for k := range s.tables {
		tableNames = append(tableNames, k)
	}
	sort.Strings(tableNames)

	for _, tblQname := range tableNames {
		tbl := s.tables[tblQname]
		nodes = append(nodes, facts.NodeFact{
			Kind: facts.KindDbTable,
			URN:  tblQname,
			Props: map[string]any{
				"qname":       tblQname,
				"schema_name": tbl.schema,
				"name":        tbl.name,
			},
		})
		for _, colName := range tbl.colOrder {
			col := tbl.columns[colName]
			colURN := tblQname + "." + col.name
			nodes = append(nodes, facts.NodeFact{
				Kind: facts.KindDbColumn,
				URN:  colURN,
				Props: map[string]any{
					"qname":       colURN,
					"table_qname": tblQname,
					"name":        col.name,
					"type":        col.typ,
					"nullable":    col.nullable,
				},
			})
		}
	}

	// Sort index keys for stable output
	idxKeys := make([]string, 0, len(s.indexes))
	for k := range s.indexes {
		idxKeys = append(idxKeys, k)
	}
	sort.Strings(idxKeys)
	for _, idxQname := range idxKeys {
		tblQname := s.indexes[idxQname]
		nodes = append(nodes, facts.NodeFact{
			Kind: facts.KindDbIndex,
			URN:  idxQname,
			Props: map[string]any{
				"qname":       idxQname,
				"table_qname": tblQname,
				"kind":        "index",
			},
		})
	}

	for _, fk := range s.fks {
		// Skip if either endpoint table is unknown
		if _, ok := s.tables[fk.srcTable]; !ok {
			continue
		}
		if _, ok := s.tables[fk.dstTable]; !ok {
			continue
		}
		edges = append(edges, facts.EdgeFact{
			Kind:    facts.EdgeFK,
			SrcKind: facts.KindDbColumn,
			SrcURN:  fk.srcTable + "." + fk.srcCol,
			DstKind: facts.KindDbColumn,
			DstURN:  fk.dstTable + "." + fk.dstCol,
		})
	}

	return nodes, edges, nil
}
