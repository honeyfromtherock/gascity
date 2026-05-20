// Package transform converts NodeFact + EdgeFact into per-table Parquet
// files on disk for the loader to COPY FROM.
package transform

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// Writers buffer facts in memory, then flush one Parquet per table.
//
// Memory budget: for Phase 1 sizes (a few million rows max) RAM is fine.
// On gridbase-core the GORM scraper alone may emit ~24M edge facts before
// reconciliation drops most of them — peak RSS during indexing has been
// observed at ~3 GB. Machines with <8 GB free RAM should not index large
// monorepos until streaming flush lands (deferred to Phase 5).
type Writers struct {
	outDir    string
	nodes     map[facts.NodeKind][]map[string]any
	edges     map[facts.EdgeKind][]map[string]any
	seenURNs  map[facts.NodeKind]map[string]bool
	seenEdges map[facts.EdgeKind]map[string]bool
}

// NewWriters constructs a Writers rooted at outDir. Call AddNode/AddEdge
// to buffer facts, then Flush to write one Parquet file per non-empty
// table under outDir/nodes/ and outDir/rels/.
func NewWriters(outDir string) *Writers {
	return &Writers{
		outDir:    outDir,
		nodes:     map[facts.NodeKind][]map[string]any{},
		edges:     map[facts.EdgeKind][]map[string]any{},
		seenURNs:  map[facts.NodeKind]map[string]bool{},
		seenEdges: map[facts.EdgeKind]map[string]bool{},
	}
}

// AddNode buffers a NodeFact for later flush. Duplicate URNs per kind are
// silently dropped — the indexer emits the same Module/File from multiple
// shards, and LadybugDB's COPY rejects duplicate primary keys.
func (w *Writers) AddNode(n facts.NodeFact) {
	seen, ok := w.seenURNs[n.Kind]
	if !ok {
		seen = map[string]bool{}
		w.seenURNs[n.Kind] = seen
	}
	if n.URN != "" {
		if seen[n.URN] {
			return
		}
		seen[n.URN] = true
	}
	w.nodes[n.Kind] = append(w.nodes[n.Kind], n.Props)
}

// manyOneEdges lists rel tables with MANY_ONE cardinality. Each source node
// may appear at most once as the FROM endpoint, so we dedup by SrcURN alone.
var manyOneEdges = map[facts.EdgeKind]bool{
	facts.EdgeDefinedIn: true,
	facts.EdgeMethodOf:  true,
	facts.EdgeHandles:   true,
	facts.EdgeFK:        true,
}

// AddEdge buffers an EdgeFact for later flush. The src/dst URNs and kinds
// are stored under reserved underscore-prefixed column names so the
// loader can resolve endpoints at COPY time.
//
// Dedup strategy:
//   - MANY_ONE rels: dedup by SrcURN — each source may appear only once.
//   - All other rels: dedup by (SrcURN, DstURN) pair.
func (w *Writers) AddEdge(e facts.EdgeFact) {
	seen, ok := w.seenEdges[e.Kind]
	if !ok {
		seen = map[string]bool{}
		w.seenEdges[e.Kind] = seen
	}
	var key string
	if manyOneEdges[e.Kind] {
		key = e.SrcURN
	} else {
		key = e.SrcURN + "\x00" + e.DstURN
	}
	if seen[key] {
		return
	}
	seen[key] = true

	row := map[string]any{}
	for k, v := range e.Props {
		row[k] = v
	}
	row["__src_urn"] = e.SrcURN
	row["__dst_urn"] = e.DstURN
	row["__src_kind"] = string(e.SrcKind)
	row["__dst_kind"] = string(e.DstKind)
	w.edges[e.Kind] = append(w.edges[e.Kind], row)
}

// Flush writes all buffered nodes and edges to Parquet files. Returns
// the first write error encountered.
func (w *Writers) Flush() error {
	if err := os.MkdirAll(filepath.Join(w.outDir, "nodes"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(w.outDir, "rels"), 0o755); err != nil {
		return err
	}
	for kind, rows := range w.nodes {
		path := filepath.Join(w.outDir, "nodes", string(kind)+".parquet")
		if err := writeParquet(path, rows); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	for kind, rows := range w.edges {
		path := filepath.Join(w.outDir, "rels", string(kind)+".parquet")
		if err := writeParquet(path, rows); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// schemaFromRow builds a parquet.Schema from the first row's key/value types.
// String columns are optional (nullable); numeric and boolean columns are
// required — LadybugDB's COPY treats optional INT32/FLOAT as BLOB.
func schemaFromRow(row map[string]any) *parquetgo.Schema {
	group := make(parquetgo.Group, len(row))
	for k, v := range row {
		node := nodeForValue(v)
		if isStringNode(v) {
			node = parquetgo.Optional(node)
		}
		group[k] = node
	}
	return parquetgo.NewSchema("row", group)
}

// isStringNode returns true when v is a string (or nil) value, i.e. the column
// should be optional (nullable) in Parquet. Numeric and boolean columns are
// written as required so LadybugDB COPY can infer their native type.
func isStringNode(v any) bool {
	if v == nil {
		return true
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.String:
		return true
	default:
		return false
	}
}

// nodeForValue returns the appropriate parquet leaf Node for a Go value.
// Nil and unknown types fall back to ByteArray (UTF-8 string).
func nodeForValue(v any) parquetgo.Node {
	if v == nil {
		return parquetgo.Leaf(parquetgo.ByteArrayType)
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.String:
		return parquetgo.Leaf(parquetgo.ByteArrayType)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return parquetgo.Leaf(parquetgo.Int32Type)
	case reflect.Int64:
		return parquetgo.Leaf(parquetgo.Int64Type)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return parquetgo.Leaf(parquetgo.Int32Type)
	case reflect.Uint64:
		return parquetgo.Leaf(parquetgo.Int64Type)
	case reflect.Float32:
		return parquetgo.Leaf(parquetgo.FloatType)
	case reflect.Float64:
		return parquetgo.Leaf(parquetgo.DoubleType)
	case reflect.Bool:
		return parquetgo.Leaf(parquetgo.BooleanType)
	default:
		return parquetgo.Leaf(parquetgo.ByteArrayType)
	}
}

// writeParquet writes rows to a Parquet file at path, building the schema
// from the first row's keys and value types (Option A: explicit schema).
func writeParquet(path string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}

	schema := schemaFromRow(rows[0])

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// GenericWriter[any] with an explicit schema gives us a stable column set
	// regardless of the dynamic map[string]any row shape.
	pw := parquetgo.NewGenericWriter[any](f, schema)

	parquetRows := make([]parquetgo.Row, len(rows))
	for i, row := range rows {
		parquetRows[i] = schema.Deconstruct(nil, row)
	}

	if _, err := pw.WriteRows(parquetRows); err != nil {
		return err
	}
	return pw.Close()
}
