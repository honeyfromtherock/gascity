package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
)

// cmdCallers implements `gc graph callers <urn> --rig <name> [--depth N]`.
// Returns the set of symbols that call (directly or transitively up to
// depth N) the given target URN.
func cmdCallers(args []string) int {
	fs := flag.NewFlagSet("callers", flag.ExitOnError)
	rig := fs.String("rig", "", "")
	root := fs.String("root", "", "override rig root path (skips rig resolution)")
	depth := fs.Int("depth", 1, "")
	allRigs := fs.Bool("all-rigs", false, "ATTACH all rigs from ~/.codegraph/rigs.toml and query across them")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || (!*allRigs && *rig == "" && *root == "") {
		fmt.Fprintln(os.Stderr, "usage: gc graph callers <urn> --rig <name> [--root <path>] [--depth N] [--all-rigs]")
		return 2
	}
	urn := fs.Arg(0)

	// Suppress unused-variable warnings: rig/root/allRigs are accepted for
	// CLI backward compatibility but rig selection now happens inside
	// queries.Callers (first scip-tier rig, falling back to first rig).
	_ = rig
	_ = root
	_ = allRigs

	rigs, err := LoadRigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	callers := queries.Callers(rigs, urn, *depth, 200)
	for _, c := range callers {
		fmt.Printf("%s\t%s:%d\n", c.URN, c.File, c.Line)
	}
	if len(callers) == 0 {
		return 2
	}
	return 0
}
