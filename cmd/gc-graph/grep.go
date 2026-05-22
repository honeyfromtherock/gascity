package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gastownhall/gascity/cmd/gc-graph/internal"
)

// cmdGrep implements `gc graph grep <symbol> --rig <name>`. Graph-first:
// tries Cypher CONTAINS scan over symbol tables; if no hits, falls back
// to ripgrep on the rig root.
func cmdGrep(args []string) int {
	fs := flag.NewFlagSet("grep", flag.ExitOnError)
	rig := fs.String("rig", "", "")
	root := fs.String("root", "", "override rig root path (skips rig resolution)")
	allRigs := fs.Bool("all-rigs", false, "ripgrep across every rig root from ~/.codegraph/rigs.toml")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || (!*allRigs && *rig == "" && *root == "") {
		fmt.Fprintln(os.Stderr, "usage: gc graph grep <symbol> --rig <name> [--root <path>] [--all-rigs]")
		return 2
	}
	sym := fs.Arg(0)

	if *allRigs {
		rigs, err := LoadRigs()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// --all-rigs runs rg per rig root rather than threading through
		// ForEachRig: grep is a filesystem op, not a graph query, and we
		// want raw file hits even on rigs that haven't been indexed yet.
		for _, r := range rigs {
			fmt.Printf("[rig=%s]\n", r.Name)
			cmd := exec.Command("rg", "-n", "--", sym, r.Root)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			_ = cmd.Run()
		}
		return 0
	}

	// Graph-first: Cypher CONTAINS scan (FTS unavailable on Ladybug v0.12.2)
	db, err := internal.OpenRig(*rig, *root)
	if err == nil {
		defer func() { _ = db.Close() }()
		c := db.Connect()
		defer func() { _ = c.Close() }()
		found := false
		for _, tbl := range []string{"Function", "Method", "Class"} {
			cypher := fmt.Sprintf(`
				MATCH (n:%s)
				WHERE LOWER(n.name) CONTAINS LOWER($q) OR LOWER(n.qname) CONTAINS LOWER($q)
				RETURN n.qname, n.file LIMIT 50`, tbl)
			rows, err := c.Query(cypher, map[string]any{"q": sym})
			if err != nil {
				continue
			}
			for rows.HasNext() {
				t, err := rows.Next()
				if err != nil {
					break
				}
				qn, _ := t.GetValue(0)
				fl, _ := t.GetValue(1)
				fmt.Printf("[graph]\t%s\t%s\n", qn, fl)
				found = true
				t.Close()
			}
			rows.Close()
		}
		if found {
			return 0
		}
	}

	// Fallback: ripgrep on the rig root. When --root is set, search that
	// directly; otherwise use the assets convention.
	var rigPath string
	if *root != "" {
		abs, _ := filepath.Abs(*root)
		rigPath = abs
	} else {
		home, _ := os.UserHomeDir()
		rigPath = fmt.Sprintf("%s/Source/grid-city/assets/%s", home, *rig)
	}
	cmd := exec.Command("rg", "-n", "--", sym, strings.TrimSpace(rigPath))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()
	return 0
}
