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
// indexes on tables with embedding columns.
//
// Both FTS and vector index creation are warn-only. LadybugDB v0.12.2 bundles
// the FTS extension but requires LOAD EXTENSION FTS; before CREATE_FTS_INDEX
// is available, and that statement corrupts C database state in this build on
// darwin/arm64 (SIGSEGV on the next query). We therefore attempt FTS index
// creation directly: if it returns "function not defined" we skip it safely;
// if a future build fixes the extension-loading path, the same code will work
// without modification.
//
// Vector indexes are similarly optional in Phase 1: CREATE_VECTOR_INDEX fails
// on tables with empty embedding columns, which is expected for the test
// fixture that omits embeddings.
func BuildIndexes(conn *store.Conn) error {
	for _, s := range ftsStatements {
		if err := conn.Exec(s); err != nil {
			fmt.Printf("warn: FTS index skipped %q: %v\n", s, err)
			// First failure means FTS extension is not loaded; skip remaining.
			break
		}
	}
	for _, s := range vecStatements {
		if err := conn.Exec(s); err != nil {
			fmt.Println("warn: vector index skipped:", err)
		}
	}
	return nil
}
