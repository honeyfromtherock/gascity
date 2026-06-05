package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gastownhall/gascity/internal/codegraph/rigdir"
	"github.com/gastownhall/gascity/internal/codegraph/store"
)

// cmdReindex implements `gc graph reindex <rig-name>` or `gc graph reindex --all`.
// It wraps invocation of gc-codegraph using the rig directory for path/profile.
func cmdReindex(args []string) int {
	fs := flag.NewFlagSet("reindex", flag.ExitOnError)
	all := fs.Bool("all", false, "reindex all registered rigs")
	binary := fs.String("bin", "./bin/gc-codegraph", "path to gc-codegraph binary")
	ifStale := fs.Bool("if-stale", false, "reindex only rigs whose git HEAD differs from the indexed :Manifest.sha (or that have no graph)")
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

	// Topo-sort: scip tier first, endpoint tier second. Within each tier preserve
	// rigs.toml order so reproducibility is by config, not by sort instability.
	sort.SliceStable(targets, func(i, j int) bool {
		return tierOrder(targets[i].Tier) < tierOrder(targets[j].Tier)
	})

	// Resolve default canonical_from once: first scip-tier rig in rigs.toml.
	defaultCanonical := ""
	for _, r := range rigs {
		if r.Tier == "scip" {
			defaultCanonical = r.Name
			break
		}
	}

	indexer, err := resolveIndexer(*binary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reindex: %v\n", err)
		return 1
	}

	failed := 0
	for _, r := range targets {
		if *ifStale && !rigStale(r) {
			fmt.Fprintf(os.Stderr, "[reindex] %s fresh — skip\n", r.Name)
			continue
		}
		sha := gitSHA(r.Root)
		out := r.Root + "/.codegraph"
		// Per-process staging dir: concurrent reindexes of the SAME rig must not
		// share one staging dir, or their interleaved writes corrupt the staged
		// graph and one swaps a partial graph.kuzu into place. Each process gets
		// its own; reap dirs leaked by dead reindexes (live-state query, not a
		// lock file). The last successful swap wins.
		cleanStaleStaging(r.Root, pidAlive)
		staging := stagingDir(r.Root, os.Getpid())
		// Build into a staging dir so the live graph at `out` stays readable
		// and an indexer failure can't destroy it. Swap on success.
		_ = os.RemoveAll(staging)
		if err := os.MkdirAll(staging, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "[reindex] %s FAILED: mkdir staging: %v\n", r.Name, err)
			failed++
			continue
		}
		args := []string{
			"--rig", r.Name,
			"--root", r.Root,
			"--out", staging,
			"--sha", sha,
		}
		if r.Profile != "" {
			args = append(args, "--profile", r.Profile)
		}
		if r.Tier == "endpoint" {
			canonical := r.CanonicalFrom
			if canonical == "" {
				canonical = defaultCanonical
			}
			if canonical != "" {
				if ageWarning := canonicalAgeWarning(rigs, canonical); ageWarning != "" {
					fmt.Fprintf(os.Stderr, "[reindex] %s: %s\n", r.Name, ageWarning)
				}
				args = append(args, "--canonical-from", canonical)
			}
		}
		fmt.Fprintf(os.Stderr, "[reindex] %s ...\n", r.Name)
		cmd := exec.Command(indexer, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[reindex] %s FAILED: %v\n", r.Name, err)
			_ = os.RemoveAll(staging) // old graph at out untouched
			failed++
			continue
		}
		if err := swapStagedGraph(out, staging); err != nil {
			fmt.Fprintf(os.Stderr, "[reindex] %s swap FAILED: %v\n", r.Name, err)
			_ = os.RemoveAll(staging)
			failed++
			continue
		}
		// New graph is live; clear superseded build intermediates from out.
		_ = os.RemoveAll(out + "/shards")
		_ = os.RemoveAll(out + "/parquet")
		_ = os.RemoveAll(out + "/csv")
		_ = os.RemoveAll(staging)
	}
	if failed > 0 {
		return 1
	}
	return 0
}

// swapStagedGraph atomically moves the freshly built graph.kuzu from staging
// into out, replacing the old graph in a single rename. graph.kuzu is a single
// file, so on the same filesystem the rename is atomic: a concurrent reader
// sees either the whole old graph or the whole new one, never a gap (unlike the
// previous delete-then-rebuild, which left the rig graphless for ~60s). Runtime
// files already in out (e.g. .last-trigger) are preserved; manifest.json is
// refreshed best-effort.
func swapStagedGraph(out, staging string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	src := filepath.Join(staging, "graph.kuzu")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("staged graph missing: %w", err)
	}
	dst := filepath.Join(out, "graph.kuzu")
	// Drop any stale WAL/tmp next to the old graph so a reader can't pair the
	// new graph with an old WAL.
	_ = os.RemoveAll(dst + ".wal")
	_ = os.RemoveAll(dst + ".tmp")
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	if b, err := os.ReadFile(filepath.Join(staging, "manifest.json")); err == nil {
		_ = os.WriteFile(filepath.Join(out, "manifest.json"), b, 0o644)
	}
	return nil
}

// resolveIndexer locates the gc-codegraph binary, independent of the current
// working directory. This matters because the Phase 5 git hook spawns the
// reindex from the rig root, where the repo-relative "./bin/gc-codegraph"
// default does not resolve.
func resolveIndexer(binFlag string) (string, error) {
	exe, _ := os.Executable()
	return resolveIndexerFrom(binFlag, exe, exec.LookPath)
}

// resolveIndexerFrom is the testable core. Resolution order:
//  1. binFlag, if it names an existing file (explicit path, or the repo's
//     ./bin/gc-codegraph default when run from the repo root)
//  2. a gc-codegraph sibling of the running executable (installed deployments
//     where gc-graph and gc-codegraph live together, e.g. ~/go/bin — required
//     for the git-hook flow)
//  3. gc-codegraph on PATH
//
// Returns an error if none resolve, so the caller can fail BEFORE deleting any
// existing graph artifacts.
func resolveIndexerFrom(binFlag, exePath string, lookPath func(string) (string, error)) (string, error) {
	if binFlag != "" {
		if _, err := os.Stat(binFlag); err == nil {
			return binFlag, nil
		}
	}
	if exePath != "" {
		sibling := filepath.Join(filepath.Dir(exePath), "gc-codegraph")
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}
	if p, err := lookPath("gc-codegraph"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("gc-codegraph not found (checked --bin %q, a sibling of %q, and PATH); pass --bin or install gc-codegraph alongside gc-graph", binFlag, exePath)
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

// tierOrder maps tier strings to a sort key. Lower values index earlier.
// Unknown tiers sort last so misconfigured rigs don't preempt canonical sources.
func tierOrder(tier string) int {
	switch tier {
	case "scip":
		return 0
	case "endpoint":
		return 1
	default:
		return 2
	}
}

// canonicalAgeWarning returns a non-empty warning string if the named rig's
// :Manifest indexed_at is older than 24h, or if the rig has no manifest at all.
// Returns "" when the manifest is fresh.
func canonicalAgeWarning(rigs []rigdir.Rig, name string) string {
	ref, err := rigdir.Lookup(rigs, name)
	if err != nil {
		return fmt.Sprintf("canonical-from rig %q not in rigs.toml", name)
	}
	graphPath := filepath.Join(ref.Root, ".codegraph", "graph.kuzu")
	if _, err := os.Stat(graphPath); err != nil {
		return fmt.Sprintf("canonical-from rig %q has no graph yet (run 'gc graph reindex %s' first)", name, name)
	}
	db, err := store.Open(graphPath, store.ModeReadOnly)
	if err != nil {
		return fmt.Sprintf("canonical-from rig %q graph unreadable: %v", name, err)
	}
	defer func() { _ = db.Close() }()
	conn := db.Connect()
	defer func() { _ = conn.Close() }()
	result, err := conn.Query(fmt.Sprintf("MATCH (m:Manifest {rig: '%s'}) RETURN m.indexed_at LIMIT 1;", name))
	if err != nil {
		return ""
	}
	defer result.Close()
	if !result.HasNext() {
		return fmt.Sprintf("canonical-from rig %q has no manifest", name)
	}
	row, err := result.Next()
	if err != nil {
		return ""
	}
	defer row.Close()
	v, err := row.GetValue(0)
	if err != nil {
		return ""
	}
	tsStr, _ := v.(string)
	if tsStr == "" {
		return ""
	}
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return ""
	}
	if time.Since(ts) > 24*time.Hour {
		return fmt.Sprintf("canonical-from rig %q indexed_at is %s old (>24h); consider 'gc graph reindex %s' first", name, time.Since(ts).Round(time.Hour), name)
	}
	return ""
}
