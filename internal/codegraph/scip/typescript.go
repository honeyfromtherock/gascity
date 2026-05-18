package scip

import (
	"fmt"
	"os/exec"
)

// TypeScriptRunner indexes TypeScript projects via @sourcegraph/scip-typescript.
type TypeScriptRunner struct{}

// Lang returns the language identifier for this runner.
func (TypeScriptRunner) Lang() string { return "typescript" }

// Run invokes scip-typescript via bun in projectDir and writes the SCIP index to indexOut.
func (TypeScriptRunner) Run(projectDir, indexOut string) error {
	cmd := exec.Command("bun", "x", "@sourcegraph/scip-typescript", "index",
		"--cwd", ".", "--output", indexOut)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scip-typescript in %s: %w\n%s", projectDir, err, out)
	}
	return nil
}
