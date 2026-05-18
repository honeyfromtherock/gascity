package scip_test

import (
	"os/exec"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/scip"
)

func TestScipTS_ProducesIndex(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not installed")
	}
	r := scip.TypeScriptRunner{}
	out := t.TempDir() + "/index.scip"
	if err := r.Run("testdata/tinyts", out); err != nil {
		t.Fatalf("run: %v", err)
	}
}
