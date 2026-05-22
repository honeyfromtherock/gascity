// Package load applies a Parquet shard tree to a fresh LadybugDB graph.
package load

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/gastownhall/gascity/internal/codegraph/schema"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// nodeOrder controls the order in which node tables are loaded.
// Nodes must be loaded before rels so PK HashIndex lookups succeed.
var nodeOrder = []string{
	"File", "Module", "Commit",
	"Function", "Method", "Class", "Interface", "Field", "Test", "Endpoint",
	"DbTable", "DbColumn", "DbIndex",
	"Manifest",
}

// relOrder controls the order in which rel tables are loaded.
var relOrder = []string{
	"CALLS", "REFERENCES", "IMPLEMENTS", "EXTENDS",
	"DEFINED_IN", "METHOD_OF", "DECLARES", "IMPORTS", "TESTS",
	"HANDLES", "CALLS_EP", "MODIFIED_BY", "COCHANGES",
	"FK", "READS_COL", "WRITES_COL",
}

// nodeColumns defines the DDL column order for each node table. This must
// match the CREATE NODE TABLE statements in schema.go. LadybugDB COPY maps
// columns by position, not name.
var nodeColumns = map[string][]string{
	"File":      {"path", "lang", "sha", "loc"},
	"Module":    {"urn", "name", "lang"},
	"Commit":    {"sha", "author", "ts"},
	"Function":  {"urn", "name", "qname", "file", "start_line", "start_col", "end_line", "end_col", "signature", "doc", "visibility", "embedding"},
	"Method":    {"urn", "name", "qname", "file", "start_line", "start_col", "end_line", "end_col", "signature", "doc", "receiver", "visibility", "embedding"},
	"Class":     {"urn", "name", "qname", "file", "start_line", "start_col", "is_interface", "doc", "embedding"},
	"Interface": {"urn", "name", "qname", "file", "start_line", "start_col", "doc", "embedding"},
	"Field":     {"urn", "name", "type", "file", "line", "owner_urn"},
	"Test":      {"urn", "name", "file", "start_line", "framework"},
	"Endpoint":  {"urn", "transport", "route", "verb"},
	"DbTable":   {"qname", "schema_name", "name"},
	"DbColumn":  {"qname", "table_qname", "name", "type", "nullable"},
	"DbIndex":   {"qname", "table_qname", "kind"},
	"Manifest":  {"rig", "sha", "profile", "indexed_at", "indexer_version", "tier"},
}

// relColumns defines the non-meta columns for each rel table (edge properties
// only; FROM/TO are resolved from __src_urn/__dst_urn).
var relColumns = map[string][]string{
	"CALLS":       {"site_file", "site_line"},
	"REFERENCES":  {"kind"},
	"IMPLEMENTS":  {},
	"EXTENDS":     {},
	"DEFINED_IN":  {},
	"METHOD_OF":   {},
	"DECLARES":    {},
	"IMPORTS":     {},
	"TESTS":       {},
	"HANDLES":     {},
	"CALLS_EP":    {"site_file", "site_line"},
	"MODIFIED_BY": {},
	"COCHANGES":   {"weight"},
	"FK":          {},
	"READS_COL":   {},
	"WRITES_COL":  {},
}

// Full applies the schema (idempotent) and bulk-loads every Parquet shard
// under parquetDir. The caller is responsible for opening db with
// ModeReadWrite and holding the RW lock exclusively.
func Full(db *store.DB, parquetDir string, prof schema.Profile) error {
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	if err := schema.Apply(conn.Inner(), prof); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "codegraph-load-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for _, t := range nodeOrder {
		p := filepath.Join(parquetDir, "nodes", t+".parquet")
		if err := loadNodeIfExists(conn, t, p, tmpDir); err != nil {
			return err
		}
	}

	for _, t := range relOrder {
		p := filepath.Join(parquetDir, "rels", t+".parquet")
		if err := loadRelIfExists(conn, t, p, tmpDir); err != nil {
			return err
		}
	}
	if err := conn.Exec("CHECKPOINT;"); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	if err := BuildIndexes(conn); err != nil {
		return err
	}
	return nil
}

// loadNodeIfExists reads a node Parquet shard and COPYs it via CSV.
// LadybugDB's Parquet COPY maps by position rather than name, so we
// materialize a CSV (in DDL column order) and COPY FROM CSV instead.
func loadNodeIfExists(conn *store.Conn, table, path, tmpDir string) error {
	if _, err := os.Stat(path); err != nil {
		return nil // shard absent, skip
	}
	cols, ok := nodeColumns[table]
	if !ok {
		return fmt.Errorf("unknown node table %q", table)
	}
	rows, err := readParquetRows(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", table, err)
	}
	if len(rows) == 0 {
		return nil
	}
	csvPath, err := writeTempCSV(tmpDir, table, cols, rows)
	if err != nil {
		return fmt.Errorf("csv %s: %w", table, err)
	}
	q := fmt.Sprintf("COPY %s FROM '%s' (HEADER=false, PARALLEL=FALSE);", table, csvPath)
	pre := countRows(conn, table)
	if err := conn.Exec(q); err != nil {
		return fmt.Errorf("copy node %s: %w", table, err)
	}
	post := countRows(conn, table)
	logCopyResult(table, pre, post, len(rows))
	return nil
}

// loadRelIfExists handles rel tables, which may have multiple (FROM,TO) type
// pairs in one Parquet file. It groups by (src_kind, dst_kind) and issues one
// COPY per pair via CSV.
func loadRelIfExists(conn *store.Conn, table, path, tmpDir string) error {
	if _, err := os.Stat(path); err != nil {
		return nil // shard absent, skip
	}
	props, ok := relColumns[table]
	if !ok {
		return fmt.Errorf("unknown rel table %q", table)
	}

	rows, err := readParquetRows(path)
	if err != nil {
		return fmt.Errorf("read rel %s: %w", table, err)
	}
	if len(rows) == 0 {
		return nil
	}

	// Group rows by (src_kind, dst_kind).
	type pairKey struct{ src, dst string }
	groups := map[pairKey][]map[string]any{}
	var pairOrder []pairKey
	for _, row := range rows {
		src, _ := row["__src_kind"].(string)
		dst, _ := row["__dst_kind"].(string)
		if src == "" || dst == "" {
			continue
		}
		k := pairKey{src, dst}
		if _, seen := groups[k]; !seen {
			pairOrder = append(pairOrder, k)
		}
		groups[k] = append(groups[k], row)
	}

	// For each pair, write a temp CSV (from, to, props…) and COPY.
	// LadybugDB COPY for multi-pair rels: "COPY Rel FROM 'f.csv' (FROM='Src', TO='Dst');"
	// CSV columns must be: from, to, [prop columns in DDL order]
	for i, pair := range pairOrder {
		pairRows := groups[pair]
		// CSV columns: from (src PK), to (dst PK), then rel properties
		cols := append([]string{"from", "to"}, props...)

		// Build rows: resolve from/to URNs and edge props.
		csvRows := make([]map[string]any, 0, len(pairRows))
		for _, row := range pairRows {
			m := map[string]any{
				"from": row["__src_urn"],
				"to":   row["__dst_urn"],
			}
			for _, p := range props {
				m[p] = row[p]
			}
			csvRows = append(csvRows, m)
		}

		csvPath, err := writeTempCSV(tmpDir, fmt.Sprintf("%s_%d", table, i), cols, csvRows)
		if err != nil {
			return fmt.Errorf("csv rel %s %s→%s: %w", table, pair.src, pair.dst, err)
		}
		q := fmt.Sprintf("COPY %s FROM '%s' (FROM='%s', TO='%s', HEADER=false, PARALLEL=FALSE, IGNORE_ERRORS=true);",
			table, csvPath, pair.src, pair.dst)
		pairLabel := fmt.Sprintf("%s (%s→%s)", table, pair.src, pair.dst)
		pre := countRows(conn, table)
		if err := conn.Exec(q); err != nil {
			// IGNORE_ERRORS handles missing-FK rows; if we still error, log
			// and continue rather than failing the entire load. Cross-package
			// references to unresolved Method/Function URNs are expected when
			// the SCIP index doesn't include the definition's document.
			fmt.Printf("warn: copy rel %s (%s→%s): %v\n", table, pair.src, pair.dst, err)
			continue
		}
		post := countRows(conn, table)
		logCopyResult(pairLabel, pre, post, len(pairRows))
	}
	return nil
}

// readParquetRows opens a Parquet file and returns all rows as maps.
func readParquetRows(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	pf, err := parquetgo.OpenFile(f, info.Size())
	if err != nil {
		return nil, err
	}

	s := pf.Schema()
	reader := parquetgo.NewGenericReader[any](pf)
	defer func() { _ = reader.Close() }()

	numRows := int(pf.NumRows())
	if numRows == 0 {
		return nil, nil
	}
	rawRows := make([]parquetgo.Row, numRows)
	n, err := reader.ReadRows(rawRows)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	result := make([]map[string]any, 0, n)
	for _, raw := range rawRows[:n] {
		m := map[string]any{}
		if err := s.Reconstruct(&m, raw); err != nil {
			return nil, fmt.Errorf("reconstruct: %w", err)
		}
		result = append(result, m)
	}
	return result, nil
}

// writeTempCSV writes rows (keyed by column name) to a CSV file in the given
// column order. Returns the file path.
func writeTempCSV(dir, name string, cols []string, rows []map[string]any) (string, error) {
	path := filepath.Join(dir, name+".csv")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)
	record := make([]string, len(cols))
	for _, row := range rows {
		for i, col := range cols {
			record[i] = valueToCSV(row[col])
		}
		if err := w.Write(record); err != nil {
			return "", err
		}
	}
	w.Flush()
	return path, w.Error()
}

// valueToCSV converts a Go value to its CSV string representation.
func valueToCSV(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// countRows returns the row count of a node or rel table. Returns 0 on error
// (silent fallback; the loader log will surface real issues).
func countRows(conn *store.Conn, table string) int {
	// Try rel-table form first.
	q := fmt.Sprintf("MATCH ()-[r:%s]->() RETURN count(r);", table)
	result, err := conn.Query(q)
	if err != nil {
		// Try node-table form.
		q = fmt.Sprintf("MATCH (n:%s) RETURN count(n);", table)
		result, err = conn.Query(q)
		if err != nil {
			return 0
		}
	}
	defer result.Close()
	if !result.HasNext() {
		return 0
	}
	row, err := result.Next()
	if err != nil {
		return 0
	}
	defer row.Close()
	v, err := row.GetValue(0)
	if err != nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// logCopyResult emits [load] N: copied X, skipped Y per COPY, with WARNING
// thresholds. If expectedRows is unknown (pass -1), only logs copied count.
func logCopyResult(table string, pre, post, expectedRows int) {
	copied := post - pre
	if expectedRows < 0 {
		log.Printf("[load] %s: copied %d", table, copied)
		return
	}
	skipped := expectedRows - copied
	if skipped < 0 {
		skipped = 0
	}
	log.Printf("[load] %s: copied %d, skipped %d", table, copied, skipped)
	if skipped > 0 {
		log.Printf("[load] WARNING: %s skipped %d rows", table, skipped)
		if expectedRows > 0 && float64(skipped) > 0.5*float64(expectedRows) {
			log.Printf("[load] WARNING: %s dropped >50%% of input rows — check schema or URN reconciliation", table)
		}
	}
}
