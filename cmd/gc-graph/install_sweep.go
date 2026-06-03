package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/agents"
)

const sweepPlistName = "com.gridbase.codegraph.sweep.plist"

// sweepPlist returns the LaunchAgent plist with all three tokens substituted.
func sweepPlist(gcGraphPath string, interval int, logPath string) string {
	s := string(agents.SweepPlist())
	s = strings.ReplaceAll(s, "__GC_GRAPH__", gcGraphPath)
	s = strings.ReplaceAll(s, "__INTERVAL__", strconv.Itoa(interval))
	s = strings.ReplaceAll(s, "__LOG__", logPath)
	return s
}

// sweepLogPath is ~/.codegraph/sweep.log (absolute). Falls back to a relative
// path only if the home dir is somehow unavailable.
func sweepLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".codegraph", "sweep.log")
	}
	return filepath.Join(home, ".codegraph", "sweep.log")
}

// installSweepAgent writes and (re)loads the launchd LaunchAgent that runs the
// periodic sweep. It is gas-city-independent and best-effort: if launchctl is
// unavailable (non-macOS) or the load fails, it prints a warning and returns
// nil so the overall install still succeeds (the sweep command remains usable
// for manual or cron invocation).
func installSweepAgent(gcGraphPath string, interval int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home dir: %w", err)
	}
	// Ensure the log dir exists.
	logPath := sweepLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("mkdir codegraph home: %w", err)
	}
	// Write the plist atomically into ~/Library/LaunchAgents.
	agentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir LaunchAgents: %w", err)
	}
	plistPath := filepath.Join(agentsDir, sweepPlistName)
	if err := writePlistAtomic(plistPath, sweepPlist(gcGraphPath, interval, logPath)); err != nil {
		return err
	}
	// Reload: unload (ignore error — nothing loaded on first install) then load.
	if _, err := exec.LookPath("launchctl"); err != nil {
		fmt.Fprintf(os.Stderr, "sweep: launchctl not found — wrote %s but did not load it (non-macOS?); run the sweep manually or via cron\n", plistPath)
		return nil
	}
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	if out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "sweep: launchctl load failed (%v): %s — plist written to %s\n", err, strings.TrimSpace(string(out)), plistPath)
		return nil
	}
	return nil
}

// writePlistAtomic writes content to path via temp + rename.
func writePlistAtomic(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
