// Package scip runs language-specific SCIP indexers and parses their output.
package scip

// Runner runs a SCIP indexer for a single language in a project directory,
// writing a SCIP protobuf file to indexOut.
type Runner interface {
	Lang() string
	Run(projectDir, indexOut string) error
}
