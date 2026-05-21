package manifest

import (
	"strings"
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
	indexedAt, ok := n.Props["indexed_at"].(string)
	if !ok {
		t.Fatalf("indexed_at should be a string, got %T", n.Props["indexed_at"])
	}
	if !strings.HasPrefix(indexedAt, "2026-05-21T12:00:00") {
		t.Errorf("indexed_at = %q, want prefix 2026-05-21T12:00:00", indexedAt)
	}
}
