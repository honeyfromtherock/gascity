package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

// quietPeriod is how long reindex-debounced waits for the commit flurry to
// settle before reindexing. A later trigger arriving during this window
// supersedes the current run (newest-wins coalescing).
const quietPeriod = 30 * time.Second

// cmdReindexDebounced implements `gc graph reindex-debounced <rig>`.
// Designed to be spawned detached by a git hook: it writes a monotonic
// trigger timestamp, waits the quiet period, and reindexes only if no newer
// trigger arrived in the meantime. A flurry of N commits spawns N processes;
// only the newest reindexes, covering all N commits.
func cmdReindexDebounced(args []string) int {
	fs := flag.NewFlagSet("reindex-debounced", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: gc graph reindex-debounced <rig>")
		return 2
	}
	rigName := fs.Arg(0)

	rigsPath, err := rigdir.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rigdir: %v\n", err)
		return 1
	}
	rigs, err := rigdir.Load(rigsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rigdir: %v\n", err)
		return 1
	}
	rig, err := rigdir.Lookup(rigs, rigName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	now := time.Now().UnixNano()
	reindex := func() error {
		self, err := os.Executable()
		if err != nil {
			self = os.Args[0]
		}
		cmd := exec.Command(self, "reindex", rigName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	did, err := runDebounced(rig.Root, now, quietPeriod, time.Sleep, reindex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reindex-debounced %s: %v\n", rigName, err)
		return 1
	}
	if did {
		fmt.Fprintf(os.Stderr, "[reindex-debounced] %s reindexed\n", rigName)
	}
	return 0
}

// runDebounced is the testable core: write `now` as the trigger timestamp,
// sleep the quiet period, then reindex iff the trigger file still holds `now`
// (i.e. no newer commit superseded us). Returns whether a reindex ran.
func runDebounced(
	rigRoot string,
	now int64,
	quiet time.Duration,
	sleep func(time.Duration),
	reindex func() error,
) (bool, error) {
	cg := filepath.Join(rigRoot, ".codegraph")
	if err := os.MkdirAll(cg, 0o755); err != nil {
		return false, fmt.Errorf("mkdir .codegraph: %w", err)
	}
	triggerPath := filepath.Join(cg, ".last-trigger")
	if err := writeTrigger(triggerPath, now); err != nil {
		return false, err
	}

	sleep(quiet)

	latest, err := readTrigger(triggerPath)
	if err != nil {
		return false, err
	}
	if latest != now {
		return false, nil
	}
	if err := reindex(); err != nil {
		return false, err
	}
	return true, nil
}

// writeTrigger atomically writes the monotonic timestamp to the trigger file
// (temp + rename, per the codebase's atomic-write convention).
func writeTrigger(path string, ts int64) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatInt(ts, 10)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readTrigger(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(b), 10, 64)
}
