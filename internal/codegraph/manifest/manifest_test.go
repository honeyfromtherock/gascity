package manifest

import (
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/facts"
)

func TestEmitProducesManifestNodeFact(t *testing.T) {
	m := Manifest{
		Rig:            "gridbase-core",
		SHA:            "abc1234",
		Profile:        "core",
		IndexedAt:      time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		IndexerVersion: "0.2.0",
		Tier:           "scip",
	}
	n := m.Emit()
	if n.Kind != facts.KindManifest {
		t.Errorf("kind = %q, want Manifest", n.Kind)
	}
	if n.URN != "gridbase-core" {
		t.Errorf("URN = %q, want gridbase-core", n.URN)
	}
	if n.Props["sha"] != "abc1234" || n.Props["tier"] != "scip" {
		t.Errorf("props = %+v", n.Props)
	}
}
