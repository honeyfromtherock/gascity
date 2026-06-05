package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const stagingPrefix = ".codegraph.staging."

// stagingDir returns this process's private staging directory for a rig. Using
// the pid keeps concurrent reindexes of the SAME rig from sharing one staging
// dir, whose interleaved writes corrupted the graph (one process swapped a
// partial graph.kuzu into place). Each process builds in isolation; the last
// successful atomic swap wins.
func stagingDir(rigRoot string, pid int) string {
	return filepath.Join(rigRoot, stagingPrefix+strconv.Itoa(pid))
}

// pidAlive reports whether a process with the given pid is currently running.
// Used to reap staging dirs leaked by killed/crashed reindexes — querying live
// process state rather than tracking it in a lock/status file. signal 0 does
// error-checking without delivering a signal; EPERM means the process exists
// but is owned by another user (still alive).
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// cleanStaleStaging removes any .codegraph.staging.<pid> dirs under rigRoot
// whose owning process is no longer alive (leaked by a killed reindex). A live
// process's staging dir, and any dir not suffixed with a numeric pid, is left
// untouched.
func cleanStaleStaging(rigRoot string, alive func(int) bool) {
	matches, _ := filepath.Glob(filepath.Join(rigRoot, stagingPrefix+"*"))
	for _, m := range matches {
		pidStr := strings.TrimPrefix(filepath.Base(m), stagingPrefix)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue // not a pid-suffixed staging dir
		}
		if !alive(pid) {
			_ = os.RemoveAll(m)
		}
	}
}
