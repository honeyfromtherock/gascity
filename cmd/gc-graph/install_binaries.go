package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// installBinaries copies the gc-graph and gc-codegraph binaries into bindir so
// they are on PATH together (the git hook guards with `command -v gc-graph`,
// and reindex resolves gc-codegraph as a sibling of gc-graph). Each copy is
// atomic (temp + rename) and chmod 0o755. A source whose absolute path already
// equals its destination is skipped (already installed in place).
func installBinaries(bindir, selfPath, indexerPath string) error {
	if err := os.MkdirAll(bindir, 0o755); err != nil {
		return err
	}
	for _, b := range []struct{ src, name string }{
		{selfPath, "gc-graph"},
		{indexerPath, "gc-codegraph"},
	} {
		dst := filepath.Join(bindir, b.name)
		if sameFile(b.src, dst) {
			continue
		}
		if err := copyExecutable(b.src, dst); err != nil {
			return fmt.Errorf("install %s: %w", b.name, err)
		}
	}
	return nil
}

// sameFile reports whether two paths resolve to the same absolute location.
func sameFile(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && aa == bb
}

// copyExecutable atomically copies src to dst with mode 0o755.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	return os.Chmod(dst, 0o755)
}

// dirOnPath reports whether dir appears in the PATH environment variable.
func dirOnPath(dir string) bool {
	abs, _ := filepath.Abs(dir)
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if pa, err := filepath.Abs(p); err == nil && pa == abs {
			return true
		}
	}
	return false
}
