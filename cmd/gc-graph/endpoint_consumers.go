package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gastownhall/gascity/internal/codegraph/queries"
)

// cmdEndpointConsumers implements `gc graph endpoint-consumers --urn <urn>`.
// Delegates cross-rig query to queries.EndpointConsumers and formats the
// result as a tabular report.
func cmdEndpointConsumers(args []string) int {
	fs := flag.NewFlagSet("endpoint-consumers", flag.ExitOnError)
	urn := fs.String("urn", "", "Canonical endpoint URN, e.g. endpoint:auth.Login")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *urn == "" {
		fmt.Fprintln(os.Stderr, "usage: gc graph endpoint-consumers --urn <urn>")
		return 2
	}

	rigs, err := LoadRigs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(rigs) == 0 {
		fmt.Fprintln(os.Stderr, "no rigs registered in ~/.codegraph/rigs.toml")
		return 1
	}

	consumers := queries.EndpointConsumers(rigs, *urn)
	fmt.Fprintf(os.Stdout, "%-25s %-9s %-60s %s\n", "RIG", "TIER", "CALLER", "LINE")
	for _, c := range consumers {
		fmt.Fprintf(os.Stdout, "%-25s %-9s %-60s %d\n", c.Rig, c.Tier, c.Caller, c.Line)
	}
	if len(consumers) == 0 {
		return 2
	}
	return 0
}
