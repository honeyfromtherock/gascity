package load

import (
	"fmt"

	"github.com/gastownhall/gascity/internal/codegraph/store"
)

var ftsStatements = []string{
	`CALL CREATE_FTS_INDEX('Function','fn_fts',['name','qname','doc']);`,
	`CALL CREATE_FTS_INDEX('Method',  'mt_fts',['name','qname','doc']);`,
	`CALL CREATE_FTS_INDEX('Class',   'cl_fts',['name','qname','doc']);`,
	`CALL CREATE_FTS_INDEX('File',    'file_fts',['path']);`,
}

var vecStatements = []string{
	`CALL CREATE_VECTOR_INDEX('Function','fn_vec','embedding', mu := 30, ml := 60);`,
	`CALL CREATE_VECTOR_INDEX('Method',  'mt_vec','embedding', mu := 30, ml := 60);`,
}

// BuildIndexes creates FTS indexes on name-bearing tables and HNSW vector
// indexes on tables with embedding columns. Both FTS and vector index
// failures are printed as warnings and skipped rather than aborting the
// load. This matches KuzuDB bundled deployments where extension
// availability varies by platform and version.
func BuildIndexes(conn *store.Conn) error {
	for _, s := range ftsStatements {
		if _, err := conn.Exec(s); err != nil {
			fmt.Printf("warn: fts index skipped %q: %v\n", s, err)
			// First failure means FTS extension is not loaded; skip remaining.
			break
		}
	}
	for _, s := range vecStatements {
		if _, err := conn.Exec(s); err != nil {
			fmt.Println("warn: vector index skipped:", err)
		}
	}
	return nil
}
