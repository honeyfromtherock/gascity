package scip

import (
	"fmt"
	"os/exec"
)

// GoRunner indexes Go projects via the scip-go binary.
type GoRunner struct{}

// Lang returns the language identifier for this runner.
func (GoRunner) Lang() string { return "go" }

// Run invokes scip-go in projectDir and writes the SCIP index to indexOut.
func (GoRunner) Run(projectDir, indexOut string) error {
	cmd := exec.Command("scip-go", "--module-root", ".", "--output", indexOut)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scip-go in %s: %w\n%s", projectDir, err, out)
	}
	return nil
}
