// Package schema defines the codegraph DDL and applies it to a Ladybug
// connection. Profiles select which optional subgraphs are included.
package schema

import (
	"fmt"
	"strings"

	ladybug "github.com/ladybugdb/go-ladybug"
)

// Profile picks which optional tables are included.
type Profile int

// Profile constants select which DDL subset is applied.
const (
	ProfileBase Profile = iota // every rig
	ProfileCore                // adds SQL/Atlas subgraph (gridbase-core only)
)

// Apply runs the DDL statements for the given profile on conn.
// Idempotent: uses CREATE NODE/REL TABLE IF NOT EXISTS.
func Apply(conn *ladybug.Connection, p Profile) error {
	for _, stmt := range statements(p) {
		res, err := conn.Query(stmt)
		if err != nil {
			return fmt.Errorf("ddl %q: %w", first80(stmt), err)
		}
		res.Close()
	}
	return nil
}

func statements(p Profile) []string {
	stmts := append([]string{}, baseDDL...)
	if p == ProfileCore {
		stmts = append(stmts, coreDDL...)
	}
	return stmts
}

func first80(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}

var baseDDL = []string{
	`CREATE NODE TABLE IF NOT EXISTS File(path STRING PRIMARY KEY, lang STRING, sha STRING, loc INT32);`,
	`CREATE NODE TABLE IF NOT EXISTS Module(urn STRING PRIMARY KEY, name STRING, lang STRING);`,
	`CREATE NODE TABLE IF NOT EXISTS Commit(sha STRING PRIMARY KEY, author STRING, ts TIMESTAMP);`,
	`CREATE NODE TABLE IF NOT EXISTS Function(urn STRING PRIMARY KEY, name STRING, qname STRING,
		file STRING, start_line INT32, start_col INT32, end_line INT32, end_col INT32,
		signature STRING, doc STRING, visibility STRING, embedding FLOAT[512]);`,
	`CREATE NODE TABLE IF NOT EXISTS Method(urn STRING PRIMARY KEY, name STRING, qname STRING,
		file STRING, start_line INT32, start_col INT32, end_line INT32, end_col INT32,
		signature STRING, doc STRING, receiver STRING, visibility STRING, embedding FLOAT[512]);`,
	`CREATE NODE TABLE IF NOT EXISTS Class(urn STRING PRIMARY KEY, name STRING, qname STRING,
		file STRING, start_line INT32, start_col INT32, is_interface BOOLEAN, doc STRING, embedding FLOAT[512]);`,
	`CREATE NODE TABLE IF NOT EXISTS Interface(urn STRING PRIMARY KEY, name STRING, qname STRING,
		file STRING, start_line INT32, start_col INT32, doc STRING, embedding FLOAT[512]);`,
	`CREATE NODE TABLE IF NOT EXISTS Field(urn STRING PRIMARY KEY, name STRING, type STRING,
		file STRING, line INT32, owner_urn STRING);`,
	`CREATE NODE TABLE IF NOT EXISTS Test(urn STRING PRIMARY KEY, name STRING,
		file STRING, start_line INT32, framework STRING);`,
	`CREATE NODE TABLE IF NOT EXISTS Endpoint(urn STRING PRIMARY KEY,
		transport STRING, route STRING, verb STRING);`,
	// `profile` is quoted because Ladybug treats PROFILE as a reserved keyword.
	"CREATE NODE TABLE IF NOT EXISTS Manifest (" +
		"rig STRING, sha STRING, `profile` STRING, " +
		"indexed_at STRING, indexer_version STRING, tier STRING, " +
		"PRIMARY KEY (rig));",
	`CREATE REL TABLE IF NOT EXISTS CALLS(
		FROM Function TO Function, FROM Function TO Method,
		FROM Method TO Function,   FROM Method TO Method,
		FROM Test TO Function,     FROM Test TO Method,
		site_file STRING, site_line INT32, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS REFERENCES(
		FROM Function TO Class, FROM Function TO Interface, FROM Function TO Field,
		FROM Method TO Class,   FROM Method TO Interface,   FROM Method TO Field,
		kind STRING, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS IMPLEMENTS(FROM Class TO Interface, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS EXTENDS(FROM Class TO Class, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS DEFINED_IN(
		FROM Function TO File, FROM Method TO File, FROM Class TO File,
		FROM Interface TO File, FROM Field TO File, FROM Test TO File, MANY_ONE);`,
	`CREATE REL TABLE IF NOT EXISTS METHOD_OF(FROM Method TO Class, FROM Method TO Interface, MANY_ONE);`,
	`CREATE REL TABLE IF NOT EXISTS DECLARES(
		FROM Module TO Function, FROM Module TO Method,
		FROM Module TO Class, FROM Module TO Interface, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS IMPORTS(FROM File TO Module, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS TESTS(FROM Test TO Function, FROM Test TO Method, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS HANDLES(FROM Function TO Endpoint, FROM Method TO Endpoint, MANY_ONE);`,
	`CREATE REL TABLE IF NOT EXISTS CALLS_EP(FROM Function TO Endpoint, FROM Method TO Endpoint,
		site_file STRING, site_line INT32, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS MODIFIED_BY(FROM File TO Commit, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS COCHANGES(FROM File TO File, weight INT32, MANY_MANY);`,
}

var coreDDL = []string{
	`CREATE NODE TABLE IF NOT EXISTS DbTable(qname STRING PRIMARY KEY, schema_name STRING, name STRING);`,
	`CREATE NODE TABLE IF NOT EXISTS DbColumn(qname STRING PRIMARY KEY, table_qname STRING, name STRING, type STRING, nullable BOOLEAN);`,
	`CREATE NODE TABLE IF NOT EXISTS DbIndex(qname STRING PRIMARY KEY, table_qname STRING, kind STRING);`,
	`CREATE REL TABLE IF NOT EXISTS FK(FROM DbColumn TO DbColumn, MANY_ONE);`,
	`CREATE REL TABLE IF NOT EXISTS READS_COL(FROM Function TO DbColumn, FROM Method TO DbColumn, MANY_MANY);`,
	`CREATE REL TABLE IF NOT EXISTS WRITES_COL(FROM Function TO DbColumn, FROM Method TO DbColumn, MANY_MANY);`,
}
