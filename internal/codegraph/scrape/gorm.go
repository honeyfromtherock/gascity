// Package scrape — GORM analyzer
package scrape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// GORM walks Go files under root and emits READS_COL edges keyed against
// known table columns extracted from GORM model struct tags. Scope: Phase 1
// approximation. Recognizes:
//
//   - Models: struct types with at least one `gorm:""` tag. Table name is
//     either `(TypeName) TableName() string { return "X" }` if defined, or
//     the snake_case pluralization of the type name.
//   - Column names: from `gorm:"column:NAME"` if specified, else snake_case
//     of the field name.
//   - Call sites: `.Where(STR, ...)`, `.Order(STR)`, `.Group(STR)`,
//     `.Joins(STR, ...)`, `.Pluck(STR, ...)`, `.Select(STR)`, `.Having(STR)`
//     where STR is a string literal containing one or more known column names.
//
// For each column reference, emits one READS_COL edge per table that owns
// that column. Phase 2 can narrow via `.Model(&T{})` / `.Table("foo")`
// resolution.
func GORM(root, schemaName string) ([]facts.NodeFact, []facts.EdgeFact, error) {
	tables, columnIndex, err := gormScanModels(root)
	if err != nil {
		return nil, nil, err
	}
	var edges []facts.EdgeFact
	if len(tables) == 0 {
		return nil, edges, nil
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
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
				if !gormIsReadMethod(method) {
					return true
				}
				if len(call.Args) < 1 {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				sql := strings.Trim(lit.Value, "`\"")
				for _, col := range gormExtractColRefs(sql) {
					for _, tbl := range columnIndex[col] {
						edges = append(edges, facts.EdgeFact{
							Kind:    facts.EdgeReadsCol,
							SrcKind: srcKind,
							SrcURN:  srcURN,
							DstKind: facts.KindDbColumn,
							DstURN:  schemaName + "." + tbl + "." + col,
						})
					}
				}
				return true
			})
			return true
		})
		return nil
	})
	_ = tables // used only via columnIndex
	return nil, edges, err
}

// gormIsReadMethod returns true for GORM chain methods whose first string arg
// commonly contains column names referenced for reading.
func gormIsReadMethod(m string) bool {
	switch m {
	case "Where", "Order", "Group", "Joins", "Pluck", "Select", "Having":
		return true
	}
	return false
}

// gormExtractColRefs returns bare identifier-shaped tokens from a SQL fragment,
// excluding SQL keywords. e.g. `"facility_id = ?"` → ["facility_id"].
var gormIdentRE = regexp.MustCompile(`[a-z_][a-z0-9_]*`)

func gormExtractColRefs(sql string) []string {
	tokens := gormIdentRE.FindAllString(strings.ToLower(sql), -1)
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if gormIsSQLKeyword(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func gormIsSQLKeyword(s string) bool {
	switch s {
	case "and", "or", "not", "in", "is", "null", "true", "false",
		"as", "on", "desc", "asc", "limit", "offset", "by", "from", "where",
		"order", "group", "having", "select", "join", "left", "right", "inner",
		"outer", "between", "like", "ilike", "exists":
		return true
	}
	return false
}

// gormScanModels walks all *.go files under root for struct types with gorm
// tags, and returns:
//   - tables: tableName → []colName
//   - columnIndex: colName → []tableName (reverse lookup for READS_COL emission)
func gormScanModels(root string) (map[string][]string, map[string][]string, error) {
	tables := map[string][]string{}
	columnIndex := map[string][]string{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}

		// First pass: collect TableName() overrides.
		tableOverrides := map[string]string{}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			if fn.Name.Name != "TableName" {
				continue
			}
			if fn.Body == nil {
				continue
			}
			for _, stmt := range fn.Body.List {
				ret, ok := stmt.(*ast.ReturnStmt)
				if !ok || len(ret.Results) != 1 {
					continue
				}
				lit, ok := ret.Results[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				typeName := gormReceiverTypeName(fn.Recv.List[0])
				if typeName != "" {
					tableOverrides[typeName] = strings.Trim(lit.Value, `"`)
				}
			}
		}

		// Second pass: collect struct types with at least one gorm tag.
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				// First, determine whether this struct is a GORM model (has at
				// least one gorm-tagged field).
				hasGorm := false
				for _, field := range st.Fields.List {
					if field.Tag == nil {
						continue
					}
					tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
					if tag.Get("gorm") != "" {
						hasGorm = true
						break
					}
				}
				if !hasGorm {
					continue
				}
				// Collect columns: gorm-tagged fields use explicit column name or
				// snake_case field name; untagged exported fields also map to
				// snake_case column names (GORM convention).
				var cols []string
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						continue // embedded / anonymous
					}
					fieldName := field.Names[0].Name
					if !unicode.IsUpper([]rune(fieldName)[0]) {
						continue // unexported
					}
					var colName string
					if field.Tag != nil {
						tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
						gormTag := tag.Get("gorm")
						if gormTag == "-" {
							continue // explicitly excluded
						}
						colName = gormExtractColumnTag(gormTag)
					}
					if colName == "" {
						colName = gormToSnakeCase(fieldName)
					}
					if colName != "" {
						cols = append(cols, colName)
					}
				}
				if len(cols) == 0 {
					continue
				}
				typeName := ts.Name.Name
				tableName := tableOverrides[typeName]
				if tableName == "" {
					tableName = gormPluralizeSnakeCase(typeName)
				}
				tables[tableName] = append(tables[tableName], cols...)
				for _, c := range cols {
					columnIndex[c] = append(columnIndex[c], tableName)
				}
			}
		}
		return nil
	})
	return tables, columnIndex, err
}

func gormReceiverTypeName(field *ast.Field) string {
	switch t := field.Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// gormExtractColumnTag returns the column name from a GORM struct tag, or ""
// if no `column:NAME` is specified. The tag format is semicolon-separated
// key[:value] pairs: e.g. `type:uuid;primaryKey;column:facility_id;not null`.
func gormExtractColumnTag(tag string) string {
	for _, part := range strings.Split(tag, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "column:") {
			return strings.TrimSpace(strings.TrimPrefix(part, "column:"))
		}
	}
	return ""
}

// gormToSnakeCase converts CamelCase to snake_case, matching GORM's naming
// convention. Consecutive uppercase letters (acronyms like "ID", "URL") are
// treated as a single token: "FacilityID" → "facility_id", "ID" → "id".
func gormToSnakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			// Insert underscore before this uppercase rune if:
			// - not at the start, AND
			// - previous rune was lowercase (e.g. "Facility|I"), OR
			// - next rune exists and is lowercase (end of acronym: "I|D" where D is followed by lower)
			if i > 0 {
				prevLower := unicode.IsLower(runes[i-1])
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if prevLower || (nextLower && unicode.IsUpper(runes[i-1])) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// gormPluralizeSnakeCase converts CamelCase to snake_case plural (GORM default).
// Naive: append 's', or 'es' for words ending in 's', 'x', 'z', 'sh', 'ch';
// 'y' → 'ies' when preceded by a consonant.
func gormPluralizeSnakeCase(s string) string {
	snake := gormToSnakeCase(s)
	switch {
	case strings.HasSuffix(snake, "s"), strings.HasSuffix(snake, "x"),
		strings.HasSuffix(snake, "z"), strings.HasSuffix(snake, "sh"),
		strings.HasSuffix(snake, "ch"):
		return snake + "es"
	case strings.HasSuffix(snake, "y") && len(snake) > 1 && !gormIsVowel(snake[len(snake)-2]):
		return snake[:len(snake)-1] + "ies"
	default:
		return snake + "s"
	}
}

func gormIsVowel(b byte) bool {
	return b == 'a' || b == 'e' || b == 'i' || b == 'o' || b == 'u'
}
