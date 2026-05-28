// Package queries holds Cypher query functions shared by the gc-graph CLI
// and the /v0/graph/* HTTP handlers. Each function takes the registered rig
// list, opens DBs read-only, ATTACHes when cross-rig, and returns typed
// result structs. No stdout/stderr I/O — callers (CLI / HTTP) decide
// presentation.
package queries

import "strings"

// RigInfo is the manifest snapshot of a single rig.
type RigInfo struct {
	Name           string `json:"name"`
	Tier           string `json:"tier"` // "scip" | "endpoint"
	Profile        string `json:"profile,omitempty"`
	SHA            string `json:"sha,omitempty"`
	IndexedAt      string `json:"indexed_at,omitempty"` // RFC3339
	IndexerVersion string `json:"indexer_version,omitempty"`
}

// EndpointInfo is one row in the endpoint catalog.
type EndpointInfo struct {
	URN       string `json:"urn"`
	Rig       string `json:"rig"`
	Transport string `json:"transport,omitempty"`
	Route     string `json:"route,omitempty"`
	Verb      string `json:"verb,omitempty"`
}

// ConsumerInfo is one consumer of an endpoint (file + line in some rig).
type ConsumerInfo struct {
	Rig    string `json:"rig"`
	Tier   string `json:"tier"`
	Caller string `json:"caller"`
	Line   int    `json:"line,omitempty"`
}

// BlastResult is the structured blast-radius bundle.
type BlastResult struct {
	Subject   string   `json:"subject"` // urn or resolved file path
	Symbols   []string `json:"symbols"`
	Files     []string `json:"files"`
	Tests     []string `json:"tests"`
	Endpoints []string `json:"endpoints"`
	DbColumns []string `json:"db_columns"`
}

// CallerInfo is one caller of a target URN.
type CallerInfo struct {
	URN  string `json:"urn"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

// aliasName returns a LadybugDB-safe alias for a rig name. Hyphens are
// replaced with underscores because LadybugDB's parser rejects hyphens in
// identifiers used by ATTACH ... AS and USE.
func aliasName(rigName string) string {
	return strings.ReplaceAll(rigName, "-", "_")
}
