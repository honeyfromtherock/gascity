package scip

import (
	"fmt"
	"os/exec"
)

// PythonRunner indexes Python projects via scip-python (run through bunx).
type PythonRunner struct{}

// Lang returns the language identifier for this runner.
func (PythonRunner) Lang() string { return "python" }

// Run invokes scip-python via bunx in projectDir and writes the SCIP index to indexOut.
func (PythonRunner) Run(projectDir, indexOut string) error {
	cmd := exec.Command("bunx", "@sourcegraph/scip-python", "index", ".", "--output", indexOut)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scip-python in %s: %w\n%s", projectDir, err, out)
	}
	return nil
}
