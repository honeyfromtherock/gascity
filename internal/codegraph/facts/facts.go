// Package facts defines the intermediate fact records emitted by SCIP
// parsers and scrapers, consumed by the transform package.
//
// One fact = one row in one Parquet shard = one node or one edge.
package facts

// NodeKind is the LadybugDB table name a NodeFact targets.
type NodeKind string

// Node kinds.
const (
	KindFile      NodeKind = "File"      // File node kind.
	KindModule    NodeKind = "Module"    // Module node kind.
	KindCommit    NodeKind = "Commit"    // Commit node kind.
	KindFunction  NodeKind = "Function"  // Function node kind.
	KindMethod    NodeKind = "Method"    // Method node kind.
	KindClass     NodeKind = "Class"     // Class node kind.
	KindInterface NodeKind = "Interface" // Interface node kind.
	KindField     NodeKind = "Field"     // Field node kind.
	KindTest      NodeKind = "Test"      // Test node kind.
	KindEndpoint  NodeKind = "Endpoint"  // Endpoint node kind.
	KindDbTable   NodeKind = "DbTable"   // DbTable node kind.
	KindDbColumn  NodeKind = "DbColumn"  // DbColumn node kind.
	KindDbIndex   NodeKind = "DbIndex"   // DbIndex node kind.
	KindManifest  NodeKind = "Manifest"  // Manifest node kind (rig index metadata).
)

// EdgeKind is the LadybugDB rel table name an EdgeFact targets.
type EdgeKind string

// Edge kinds.
const (
	EdgeCalls      EdgeKind = "CALLS"       // Calls edge kind.
	EdgeReferences EdgeKind = "REFERENCES"  // References edge kind.
	EdgeImplements EdgeKind = "IMPLEMENTS"  // Implements edge kind.
	EdgeExtends    EdgeKind = "EXTENDS"     // Extends edge kind.
	EdgeDefinedIn  EdgeKind = "DEFINED_IN"  // DefinedIn edge kind.
	EdgeMethodOf   EdgeKind = "METHOD_OF"   // MethodOf edge kind.
	EdgeDeclares   EdgeKind = "DECLARES"    // Declares edge kind.
	EdgeImports    EdgeKind = "IMPORTS"     // Imports edge kind.
	EdgeTests      EdgeKind = "TESTS"       // Tests edge kind.
	EdgeHandles    EdgeKind = "HANDLES"     // Handles edge kind.
	EdgeCallsEP    EdgeKind = "CALLS_EP"    // CallsEP edge kind.
	EdgeModifiedBy EdgeKind = "MODIFIED_BY" // ModifiedBy edge kind.
	EdgeCochanges  EdgeKind = "COCHANGES"   // Cochanges edge kind.
	EdgeFK         EdgeKind = "FK"          // FK edge kind.
	EdgeReadsCol   EdgeKind = "READS_COL"   // ReadsCol edge kind.
	EdgeWritesCol  EdgeKind = "WRITES_COL"  // WritesCol edge kind.
)

// NodeFact is a single node row. Props maps column name → value.
type NodeFact struct {
	Kind  NodeKind
	URN   string         // PK for every kind except File (path) and Commit (sha)
	Props map[string]any // column name → value; must match the schema's columns
}

// EdgeFact is a single rel row. SrcKind+SrcURN and DstKind+DstURN identify
// the endpoints; Props maps column name → value.
type EdgeFact struct {
	Kind    EdgeKind
	SrcKind NodeKind
	SrcURN  string
	DstKind NodeKind
	DstURN  string
	Props   map[string]any
}
