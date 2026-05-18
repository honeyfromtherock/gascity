package scip_test

import (
	"os/exec"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/scip"
)

func TestScipPy_ProducesIndex(t *testing.T) {
	if _, err := exec.LookPath("bunx"); err != nil {
		t.Skip("bunx not installed")
	}
	r := scip.PythonRunner{}
	out := t.TempDir() + "/index.scip"
	if err := r.Run("testdata/tinypy", out); err != nil {
		t.Fatalf("run: %v", err)
	}
}
