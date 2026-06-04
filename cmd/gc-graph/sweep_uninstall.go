package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// sweepPlistPath returns the absolute path of the launchd sweep LaunchAgent.
func sweepPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", sweepPlistName), nil
}

// removeIfExists removes path, treating "already absent" as success.
func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// uninstallSweepAgent unloads (best-effort) and removes the launchd sweep
// LaunchAgent. Idempotent: succeeds even if the agent was never installed.
func uninstallSweepAgent() error {
	plistPath, err := sweepPlistPath()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("launchctl"); err == nil {
		_ = exec.Command("launchctl", "unload", plistPath).Run() // best-effort; not loaded is fine
	}
	return removeIfExists(plistPath)
}

// cmdSweepUninstall implements `gc graph sweep-uninstall`.
func cmdSweepUninstall(args []string) int {
	if err := uninstallSweepAgent(); err != nil {
		fmt.Fprintf(os.Stderr, "sweep-uninstall: %v\n", err)
		return 1
	}
	plistPath, _ := sweepPlistPath()
	fmt.Fprintf(os.Stdout, "+ removed launchd sweep agent (%s)\n", plistPath)
	return 0
}
