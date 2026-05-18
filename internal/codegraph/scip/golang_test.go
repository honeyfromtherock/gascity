package scip_test

import (
	"os/exec"
	"testing"

	"github.com/gastownhall/gascity/internal/codegraph/scip"
)

func TestScipGo_ProducesIndex(t *testing.T) {
	if _, err := exec.LookPath("scip-go"); err != nil {
		t.Skip("scip-go not installed")
	}
	r := scip.GoRunner{}
	out := t.TempDir() + "/index.scip"
	if err := r.Run("testdata/tinygo", out); err != nil {
		t.Fatalf("run: %v", err)
	}
	info, err := exec.Command("stat", out).Output()
	if err != nil || len(info) == 0 {
		t.Fatalf("index file missing/empty: %v", err)
	}
}
