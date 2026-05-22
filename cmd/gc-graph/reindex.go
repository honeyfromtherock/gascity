package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
)

// cmdReindex implements `gc graph reindex <rig-name>` or `gc graph reindex --all`.
// It wraps invocation of gc-codegraph using the rig directory for path/profile.
func cmdReindex(args []string) int {
	fs := flag.NewFlagSet("reindex", flag.ExitOnError)
	all := fs.Bool("all", false, "reindex all registered rigs")
	binary := fs.String("bin", "./bin/gc-codegraph", "path to gc-codegraph binary")
	_ = fs.Parse(args)

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

	targets := rigs
	if !*all {
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: gc graph reindex <rig-name> | --all")
			return 2
		}
		name := fs.Arg(0)
		r, err := rigdir.Lookup(rigs, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		targets = []rigdir.Rig{r}
	}

	failed := 0
	for _, r := range targets {
		sha := gitSHA(r.Root)
		out := r.Root + "/.codegraph"
		// Clean prior graph artifacts to avoid duplicate-PK errors on re-load
		// and to clear stale Ladybug WAL files / shard outputs that block reopen.
		_ = os.RemoveAll(out + "/graph.kuzu")
		_ = os.RemoveAll(out + "/graph.kuzu.wal")
		_ = os.RemoveAll(out + "/graph.kuzu.tmp")
		_ = os.RemoveAll(out + "/shards")
		_ = os.RemoveAll(out + "/parquet")
		_ = os.RemoveAll(out + "/csv")
		args := []string{
			"--rig", r.Name,
			"--root", r.Root,
			"--out", out,
			"--sha", sha,
		}
		if r.Profile != "" {
			args = append(args, "--profile", r.Profile)
		}
		fmt.Fprintf(os.Stderr, "[reindex] %s ...\n", r.Name)
		cmd := exec.Command(*binary, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[reindex] %s FAILED: %v\n", r.Name, err)
			failed++
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// gitSHA returns the HEAD commit SHA for the repo at root, or "none" if it's
// not a git repo / git is unavailable.
func gitSHA(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "none"
	}
	return strings.TrimSpace(string(out))
}
