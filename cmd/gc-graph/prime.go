package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// stringSliceFlag is a repeatable string flag accumulator.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error { *s = append(*s, v); return nil }

// runPrime gathers graph context for a bead and writes a markdown summary.
// Used by the codegraph.prime formula step in gas-city orchestration.
//
// Inputs come from either --bead (shells out to `bd show <id> --json` and
// extracts props.touched_files / props.endpoint_urns) or directly via
// repeatable --touched-file / --endpoint-urn flags.
func runPrime(args []string) int {
	fs := flag.NewFlagSet("prime", flag.ExitOnError)
	beadID := fs.String("bead", "", "bead ID (looks up via `bd show <id>` for touched-files/endpoint-urns)")
	out := fs.String("out", "", "output markdown path (default stdout)")
	var touchedFiles stringSliceFlag
	var endpointURNs stringSliceFlag
	fs.Var(&touchedFiles, "touched-file", "explicit touched file (repeatable, alternative to --bead)")
	fs.Var(&endpointURNs, "endpoint-urn", "explicit endpoint URN (repeatable, alternative to --bead)")
	_ = fs.Parse(args)

	if *beadID != "" {
		tf, eu, err := loadBeadInputs(*beadID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "prime: bd show %s: %v (continuing with explicit flags only)\n", *beadID, err)
		} else {
			touchedFiles = append(touchedFiles, tf...)
			endpointURNs = append(endpointURNs, eu...)
		}
	}

	bin := os.Getenv("GC_GRAPH_BIN")
	if bin == "" {
		bin = os.Args[0]
	}

	var sb strings.Builder
	sb.WriteString("# Codegraph context\n\n")
	hadContent := false

	for _, urn := range endpointURNs {
		cmd := exec.Command(bin, "endpoint-consumers", "--urn", urn)
		o, err := cmd.Output()
		if err == nil && len(o) > 0 {
			sb.WriteString(fmt.Sprintf("## Consumers of `%s`\n\n```\n%s\n```\n\n", urn, string(o)))
			hadContent = true
		}
	}
	for _, file := range touchedFiles {
		cmd := exec.Command(bin, "blast", "--file", file, "--format", "hook", "--max-tokens", "400")
		o, err := cmd.Output()
		if err == nil && len(o) > 0 {
			sb.WriteString(fmt.Sprintf("## Blast radius of `%s`\n\n%s\n\n", file, string(o)))
			hadContent = true
		}
	}

	if !hadContent {
		sb.WriteString("(no graph context available; bead has no touched files or endpoint URNs in props)\n")
	}

	output := sb.String()
	if *out != "" {
		if err := os.WriteFile(*out, []byte(output), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "prime: write %s: %v\n", *out, err)
			return 1
		}
		return 0
	}
	fmt.Fprint(os.Stdout, output)
	return 0
}

// loadBeadInputs shells out to `bd show <id> --json` and extracts
// props.touched_files and props.endpoint_urns. Returns errors silently —
// callers fall back to explicit flags.
func loadBeadInputs(beadID string) (touched []string, urns []string, err error) {
	cmd := exec.Command("bd", "show", beadID, "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}
	var bead struct {
		Props map[string]any `json:"props"`
	}
	if err := json.Unmarshal(out, &bead); err != nil {
		return nil, nil, err
	}
	if arr, ok := bead.Props["touched_files"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				touched = append(touched, s)
			}
		}
	}
	if arr, ok := bead.Props["endpoint_urns"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				urns = append(urns, s)
			}
		}
	}
	return touched, urns, nil
}
