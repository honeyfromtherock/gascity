// Package manifest holds the per-rig index manifest emitted as a :Manifest graph node.
package manifest

import (
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

// Manifest is one rig's index metadata. Written as a single :Manifest node keyed by Rig.
type Manifest struct {
	Rig            string
	SHA            string
	Profile        string
	IndexedAt      time.Time
	IndexerVersion string
	Tier           string // "scip" or "endpoint"
}

// Emit returns a NodeFact suitable for the transform/load pipeline.
func (m Manifest) Emit() facts.NodeFact {
	return facts.NodeFact{
		Kind: facts.KindManifest,
		URN:  m.Rig,
		Props: map[string]any{
			"rig":             m.Rig,
			"sha":             m.SHA,
			"profile":         m.Profile,
			"indexed_at":      m.IndexedAt.UTC().Format(time.RFC3339),
			"indexer_version": m.IndexerVersion,
			"tier":            m.Tier,
		},
	}
}
